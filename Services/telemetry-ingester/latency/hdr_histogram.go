// Package latency provides O(1)-record latency percentile estimation.
package latency

import (
	"math"
	"math/bits"
)

const (
	// lowestDiscernibleValue is the smallest value distinguished from zero, in
	// microseconds. With a unit magnitude of 0 every microsecond up to
	// subBucketCount is tracked exactly.
	lowestDiscernibleValue = 1
	// highestTrackableValue caps the tracked range at 10 seconds in microseconds.
	// Values above are clamped to this bound.
	highestTrackableValue = 10_000_000
	// significantDigits is the number of significant decimal digits preserved.
	// Two digits gives ~1% relative precision and keeps the counts array in the
	// tens of kilobytes (about 18 KB) regardless of how many values are recorded.
	significantDigits = 2
)

// HDRHistogram records values into logarithmically sized buckets that each hold
// a fixed number of significant digits. Memory is bounded to a few thousand
// buckets (about 18 KB) no matter how many values are recorded. RecordValue is
// O(1); Percentile scans the bucket set once.
//
// The bucket layout follows the standard High Dynamic Range histogram: the
// value range is divided into power-of-two "buckets", each subdivided into a
// fixed number of linearly spaced sub-buckets. A value's index is found in
// constant time from its bit length, and the value a bucket represents is
// recovered by reversing that mapping.
type HDRHistogram struct {
	counts []int64

	subBucketCount              int32
	subBucketHalfCount          int32
	subBucketHalfCountMagnitude uint32
	subBucketMask               int64
	unitMagnitude               uint32
	leadingZeroCountBase        uint32
	countsLen                   int32

	totalCount  int64
	totalSum    int64
	maxObserved int64
	minObserved int64
}

// NewHDRHistogram constructs an empty histogram over [1µs, 10s] at two
// significant digits of precision.
func NewHDRHistogram() *HDRHistogram {
	// largestValueWithSingleUnitResolution = 2 * 10^significantDigits.
	largest := int64(2)
	for i := 0; i < significantDigits; i++ {
		largest *= 10
	}

	// unitMagnitude = floor(log2(lowestDiscernibleValue)). With value 1 this is 0.
	unitMagnitude := uint32(bits.Len64(uint64(lowestDiscernibleValue)) - 1)

	// subBucketCountMagnitude = ceil(log2(largest)), at least 1.
	subBucketCountMagnitude := uint32(bits.Len64(uint64(largest - 1)))
	if subBucketCountMagnitude < 1 {
		subBucketCountMagnitude = 1
	}
	subBucketHalfCountMagnitude := subBucketCountMagnitude - 1
	subBucketCount := int32(1) << subBucketCountMagnitude
	subBucketHalfCount := subBucketCount / 2
	subBucketMask := (int64(subBucketCount) - 1) << unitMagnitude

	h := &HDRHistogram{
		subBucketCount:              subBucketCount,
		subBucketHalfCount:          subBucketHalfCount,
		subBucketHalfCountMagnitude: subBucketHalfCountMagnitude,
		subBucketMask:               subBucketMask,
		unitMagnitude:               unitMagnitude,
		leadingZeroCountBase:        uint32(64) - unitMagnitude - subBucketCountMagnitude,
		minObserved:                 math.MaxInt64,
	}

	bucketCount := h.bucketsNeededToCover(highestTrackableValue)
	h.countsLen = (bucketCount + 1) * (subBucketCount / 2)
	h.counts = make([]int64, h.countsLen)
	return h
}

// bucketsNeededToCover returns the number of power-of-two buckets required to
// track values up to the given maximum.
func (h *HDRHistogram) bucketsNeededToCover(value int64) int32 {
	smallestUntrackable := int64(h.subBucketCount) << h.unitMagnitude
	bucketsNeeded := int32(1)
	for smallestUntrackable < value {
		if smallestUntrackable > math.MaxInt64/2 {
			return bucketsNeeded + 1
		}
		smallestUntrackable <<= 1
		bucketsNeeded++
	}
	return bucketsNeeded
}

func (h *HDRHistogram) bucketIndex(value int64) int32 {
	return int32(h.leadingZeroCountBase) - int32(bits.LeadingZeros64(uint64(value|h.subBucketMask)))
}

func (h *HDRHistogram) subBucketIndex(value int64, bucketIndex int32) int32 {
	return int32(uint64(value) >> (uint32(bucketIndex) + h.unitMagnitude))
}

func (h *HDRHistogram) countsIndexFor(bucketIndex, subBucketIndex int32) int32 {
	bucketBaseIndex := (bucketIndex + 1) << h.subBucketHalfCountMagnitude
	offsetInBucket := subBucketIndex - h.subBucketHalfCount
	return bucketBaseIndex + offsetInBucket
}

func (h *HDRHistogram) countsIndex(value int64) int32 {
	bi := h.bucketIndex(value)
	si := h.subBucketIndex(value, bi)
	return h.countsIndexFor(bi, si)
}

// valueFromIndex returns the lowest value that maps to the given counts index.
// This is the value reported for a percentile that falls in that bucket; it is
// within the histogram's relative precision of every value in the bucket.
func (h *HDRHistogram) valueFromIndex(index int32) int64 {
	bucketIndex := (index >> h.subBucketHalfCountMagnitude) - 1
	subBucketIndex := (index & (h.subBucketHalfCount - 1)) + h.subBucketHalfCount
	if bucketIndex < 0 {
		subBucketIndex -= h.subBucketHalfCount
		bucketIndex = 0
	}
	return int64(subBucketIndex) << (uint32(bucketIndex) + h.unitMagnitude)
}

// RecordValue records a latency in microseconds. O(1).
func (h *HDRHistogram) RecordValue(latencyUs int64) {
	if latencyUs < 0 {
		latencyUs = 0
	}
	if latencyUs > highestTrackableValue {
		latencyUs = highestTrackableValue
	}
	h.counts[h.countsIndex(latencyUs)]++
	h.totalCount++
	h.totalSum += latencyUs
	if latencyUs > h.maxObserved {
		h.maxObserved = latencyUs
	}
	if latencyUs < h.minObserved {
		h.minObserved = latencyUs
	}
}

// Percentile returns the latency at the given percentile (e.g. 0.99), reported
// to the histogram's relative precision.
func (h *HDRHistogram) Percentile(pct float64) int64 {
	if h.totalCount == 0 {
		return 0
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	target := int64(pct*float64(h.totalCount) + 0.5)
	if target < 1 {
		target = 1
	}
	if target > h.totalCount {
		target = h.totalCount
	}
	var cumulative int64
	for i := int32(0); i < h.countsLen; i++ {
		cumulative += h.counts[i]
		if cumulative >= target {
			return h.valueFromIndex(i)
		}
	}
	return h.maxObserved
}

// Mean returns the average latency.
func (h *HDRHistogram) Mean() float64 {
	if h.totalCount == 0 {
		return 0
	}
	return float64(h.totalSum) / float64(h.totalCount)
}

// Count returns the number of recorded values.
func (h *HDRHistogram) Count() int64 { return h.totalCount }

// Reset zeroes the histogram for reuse.
func (h *HDRHistogram) Reset() {
	for i := range h.counts {
		h.counts[i] = 0
	}
	h.totalCount = 0
	h.totalSum = 0
	h.maxObserved = 0
	h.minObserved = math.MaxInt64
}

// MergeFrom adds another histogram's counts into this one. Both histograms must
// share the same configuration (they do, since the constructor takes no
// parameters). Not concurrency-safe; the caller must hold any necessary lock.
func (h *HDRHistogram) MergeFrom(other *HDRHistogram) {
	if other == nil {
		return
	}
	n := len(h.counts)
	if len(other.counts) < n {
		n = len(other.counts)
	}
	for i := 0; i < n; i++ {
		h.counts[i] += other.counts[i]
	}
	h.totalCount += other.totalCount
	h.totalSum += other.totalSum
	if other.maxObserved > h.maxObserved {
		h.maxObserved = other.maxObserved
	}
	if other.minObserved < h.minObserved {
		h.minObserved = other.minObserved
	}
}
