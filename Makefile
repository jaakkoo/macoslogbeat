BEAT_NAME=macoslogbeat
BEAT_PATH=github.com/jaakkoo/macoslogbeat
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
SYSTEM_TESTS=false
TEST_ENVIRONMENT=false
ES_BEATS?=./vendor/github.com/elastic/beats
LIBBEAT_MAKEFILE=$(ES_BEATS)/libbeat/scripts/Makefile
GOPACKAGES=$(shell govendor list -no-status +local)
GOBUILD_FLAGS=-i -ldflags "-X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.buildTime=$(NOW) -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.commit=$(COMMIT_ID)"
MAGE_IMPORT_PATH=${BEAT_PATH}/vendor/github.com/magefile/mage
NO_COLLECT=true
CHECK_HEADERS_DISABLED=true

LOGBEAT_BINARY := $(CURDIR)/build/golang-crossbuild/macoslogbeat-darwin-amd64
LOGBEAT_CONF := $(CURDIR)/macoslogbeat.yml
LOGBEAT_FOLDER := $(CURDIR)/build/pkg
LOGBEAT_VERSION := 0.0.1
LOGBEAT_PKG := $(LOGBEAT_FOLDER)/macoslogbeat-$(LOGBEAT_VERSION).pkg

export PLATFORMS := darwin/amd64

# Path to the libbeat Makefile
-include $(LIBBEAT_MAKEFILE)

.PHONY: copy-vendor
copy-vendor:
	mage vendorUpdate

$(LOGBEAT_CONF): update

$(LOGBEAT_BINARY): $(LOGBEAT_CONF) release

$(LOGBEAT_PKG): $(LOGBEAT_BINARY)
	mkdir -p $(LOGBEAT_FOLDER)/buildroot/opt/macoslogbeat
	cp $(LOGBEAT_BINARY) $(LOGBEAT_FOLDER)/buildroot/opt/macoslogbeat/macoslogbeat
	cp $(LOGBEAT_CONF) $(LOGBEAT_FOLDER)/buildroot/opt/macoslogbeat
	pkgbuild \
		--identifier com.reaktor.macoslogbeat \
		--root $(LOGBEAT_FOLDER)/buildroot/ \
		--version $(LOGBEAT_VERSION) \
		$(LOGBEAT_PKG)

.PHONY: pkg
pkg: $(LOGBEAT_PKG)
