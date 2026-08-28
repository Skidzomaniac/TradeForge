-- Columnar compression + chunk sizing for the high-volume hypertable.
-- Wrapped so a missing capability never fails the whole migration run.
DO $$
BEGIN
  PERFORM set_chunk_time_interval('latency_samples', INTERVAL '1 hour');
  ALTER TABLE latency_samples SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'time DESC',
    timescaledb.compress_segmentby = 'contestant_id'
  );
  PERFORM add_compression_policy('latency_samples', INTERVAL '7 days');
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'compression setup skipped: %', SQLERRM;
END $$;
