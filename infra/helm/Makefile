lint-all:
	@for c in */ ; do [ -f "$$c/Chart.yaml" ] && helm lint "$$c" || true; done

package-all:
	@mkdir -p packages
	@for c in */ ; do [ -f "$$c/Chart.yaml" ] && helm package "$$c" -d packages/ || true; done
