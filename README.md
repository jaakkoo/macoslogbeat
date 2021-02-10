# MacosLogbeat

Macoslogbeat is a log shipper for macos unified logs like journalbeat is for journald.

It's only tested to work on macos Catalina (10.15), but I bet it would work on any macos release that uses unified log.

# Installing

1. Pick the latest [release](https://github.com/jaakkoo/macoslogbeat/releases)
2. Install it either by doubleclicking the pkg or with installer (`sudo installer -pkg macoslogbeat-<version>.pkg -target /`)
3. Configure `/opt/macoslogbeat/macoslogbeat.yml`.
    * Mainly elasticsearch/logstash/etc. location is required to get started

Optional steps:

4. Install profile to see <private> log fields also (located in `/opt/macososlogbeat/macos/Logging.mobileconfig`)
5. Install launchd configuration to run it automatically when os boots and restart on crash: `sudo launchctl load /opt/macoslogbeat/install/com.reaktor.macoslogbeat.plist`
6. Run the service: `sudo launchctl start com.reaktor.macoslogbeat`

# Development

### Requirements

* [Golang](https://golang.org/dl/) 1.15.7
* [mage](https://github.com/magefile/mage) 1.8.0
* [Python](https://www.python.org/downloads/) >3.7 (for config file generation)

For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).

Make sure you have ${GOPATH}/bin in PATH.

### Build

To build the binary for MacosLogbeat run the command below. This will generate a binary
in the same directory with the name macoslogbeat.

```
make
```


### Run

To run MacosLogbeat with debugging output enabled, run:

```
./macoslogbeat -c macoslogbeat.yml -e -d "*"
```

### Development with elasticsearch and kibana

Run `docker-compose up` to start elasticsearch and kibana locally to start testing macoslogbeat.

After the apps have started elasticsearch is available in `http://localhost:9200` and kibana in `http://localhost:5601`.

### Test

To test MacosLogbeat, run the following command:
(As of writing this there are no tests, but maybe this changes over time)

```
make testsuite
```

alternatively:
```
make unit-tests
make system-tests
make integration-tests
make coverage-report
```

The test coverage is reported in the folder `./build/coverage/`

### Update

Each beat has a template for the mapping in elasticsearch and a documentation for the fields
which is automatically generated based on `fields.yml` by running the following command.

```
make update
```


### Cleanup

To clean  MacosLogbeat source code, run the following command:

```
make fmt
```

To clean up the build directory and generated artifacts, run:

```
make clean
```


### Clone

To clone MacosLogbeat from the git repository, run the following commands:

```
mkdir -p ${GOPATH}/src/github.com/jaakkoo/macoslogbeat
git clone https://github.com/jaakkoo/macoslogbeat ${GOPATH}/src/github.com/jaakkoo/macoslogbeat
```


For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).


## Packaging

The beat frameworks provides tools to crosscompile and package your beat for different platforms. This requires [docker](https://www.docker.com/) and vendoring as described above. To build packages of your beat, run the following command:

```
make pkg
```

This will fetch and create all images required for the build process. The whole process to finish can take several minutes.

It will also generate Macos installer (.pkg) and it is made available in the same directory (build/) than everything
else.

## Install

Once macoslogbeat-<version>.pkg package is installed the files will be available in `/opt/macoslogbeat`

The easiest way to verify the application works is to run it manually from commandline:
`sudo /opt/macoslogbeat/macoslogbeat -c /opt/macoslogbeat/macoslogbeat.yml -e`. 

Note! Running with sudo is optional, but unless you ran it with enough priviledges most of the log messages are hidden

Get the help: `sudo /opt/macoslogbeat/macoslogbeat -h`

### Configuration
All configuration is located in `/opt/macoslogbeat/macoslogbeat.yml`

### Run automatically

The launchd configuration is available in `/opt/macoslogbeat/install/com.reaktor.macoslogbeat.plist`.

Install it: `sudo launchctl load /opt/macoslogbeat/install/com.reaktor.macoslogbeat.plist`

Start it: `sudo launchctl start com.reaktor.macoslogbeat`.

Launcd will configure `macoslogbeat` to start automatically when OS boots. If you wish to change the behaviour edit the plist.

## Uninstall

First unload the launchd config
`sudo launchctl unload /opt/macoslogbeat/install/com.reaktor.macoslogbeat.plist`

Remove the package from database
`sudo pkgutil --forget com.reaktor.macoslogbeat`

Delete all files
`rm -rf /opt/macoslogbeat`
