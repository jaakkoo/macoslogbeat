BEAT_NAME=macoslogbeat
BEAT_PATH=github.com/jaakkoo/macoslogbeat
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
SYSTEM_TESTS=false
TEST_ENVIRONMENT=false
ES_BEATS_IMPORT_PATH=github.com/elastic/beats/v7
ES_BEATS?=$(shell go list -m -f '{{.Dir}}' ${ES_BEATS_IMPORT_PATH})
LIBBEAT_MAKEFILE=$(ES_BEATS)/libbeat/scripts/Makefile
GOPACKAGES=$(shell go list ${BEAT_PATH}/... | grep -v /tools)
GOBUILD_FLAGS=-i -ldflags "-X ${ES_BEATS_IMPORT_PATH}/libbeat/version.buildTime=$(NOW) -X ${ES_BEATS_IMPORT_PATH}/libbeat/version.commit=$(COMMIT_ID)"
MAGE_IMPORT_PATH=github.com/magefile/mage
NO_COLLECT=true
CHECK_HEADERS_DISABLED=true

LOGBEAT_VERSION := 1.0.1
LOGBEAT_BINARY := $(CURDIR)/build/golang-crossbuild/macoslogbeat-darwin-amd64
LOGBEAT_FOLDER := $(CURDIR)/build/pkg
LOGBEAT_CONF := $(CURDIR)/macoslogbeat.yml
BUILDROOT := $(LOGBEAT_FOLDER)/buildroot/opt/macoslogbeat
LOGBEAT_LAUNCHCTL := $(CURDIR)/macos/com.reaktor.macoslogbeat.plist
LOGBEAT_PROFILE := $(CURDIR)/macos/Logging.mobileconfig
LOGBEAT_PKG := $(LOGBEAT_FOLDER)/macoslogbeat-$(LOGBEAT_VERSION).pkg
 
# You can build it for other platforms also, but it will not run due to the requirement to MacOS unified logging.
export PLATFORMS := darwin/amd64

# Path to the libbeat Makefile
-include $(LIBBEAT_MAKEFILE)

$(LOGBEAT_BINARY): release

$(LOGBEAT_PKG): $(LOGBEAT_BINARY) $(LOGBEAT_LAUNCHCTL) $(LOGBEAT_PROFILE) $(LOGBEAT_CONF)
	mkdir -p $(BUILDROOT)/install
	cp $(LOGBEAT_BINARY) $(BUILDROOT)/macoslogbeat
	cp $(LOGBEAT_CONF) $(BUILDROOT)
	cp $(LOGBEAT_LAUNCHCTL) $(BUILDROOT)/install/
	cp $(LOGBEAT_PROFILE) $(BUILDROOT)/install/
	pkgbuild \
		--identifier com.reaktor.macoslogbeat \
		--root $(LOGBEAT_FOLDER)/buildroot/ \
		--version $(LOGBEAT_VERSION) \
		$(LOGBEAT_PKG)

.PHONY: pkg
pkg: $(LOGBEAT_PKG)
