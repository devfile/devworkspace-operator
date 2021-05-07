# Resolve current git commit hash from either a .git directory
# or from a local file named `.commit_hash`. The latter case
# is necessary since midstream repositories might have the
# .git directory one level higher and thus outside the docker
# build context.
ifneq (,$(wildcard .git/HEAD))
REF := $(shell cat .git/HEAD | sed 's|ref: ||')
export GIT_COMMIT_ID := $(shell cat .git/$(REF))
else ifneq (,$(wildcard .commit_hash))
export GIT_COMMIT_ID := $(shell cat .commit_hash)
else
export GIT_COMMIT_ID := "unknown"
endif

# Additional build info
export GO_PACKAGE_PATH := $(shell head -n1 go.mod | cut -d " " -f 2)
export BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Architecture we're building on
export ARCH := $(shell uname -m)
ifeq (x86_64,$(ARCH))
export ARCH := amd64
else ifeq (aarch64,$(ARCH))
export ARCH := arm64
endif
