package beater

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/bcicen/jstream"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/jaakkoo/macoslogbeat/config"
)

var rfc3339ms string = "2006-01-02T15:04:05.000Z0700"

// macoslogbeat configuration.
type macoslogbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

// New creates an instance of macoslogbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &macoslogbeat{
		done:   make(chan struct{}),
		config: c,
	}
	return bt, nil
}

func buildArgs(command string, excluded []string) []string {
	args := []string{command, "--style=json"}

	if len(excluded) == 0 {
		return args
	}

	var subsystems []string
	for _, subsystem := range excluded {
		subsystems = append(subsystems, fmt.Sprintf("(subsystem != \"%s\")", subsystem))
	}

	return append(args, "--predicate", strings.Join(subsystems, " and "))
}

func writeTimestamp(timestamp time.Time, fileName string) {
	err := ioutil.WriteFile(fileName, []byte(timestamp.Format(rfc3339ms)), 0600)
	if err != nil {
		logp.Err("Could not update cache file timestamp")
	}
}

func readTimestamp(fileName string) (time.Time, error) {
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not read contents of %s", fileName)
	}
	t, err := time.Parse(rfc3339ms, string(contents))
	if err != nil {
		return time.Time{}, fmt.Errorf("Timestamp is malformed")
	}
	return t, nil
}

func publishEvent(bt *macoslogbeat, eventFields *common.MapStr) {
	ts, err := parseMacTimestamp((*eventFields)["timestamp"].(string))
	if err != nil {
		logp.Err("Cant use event timestamp, using time.Now().")
		ts = time.Now()
	}
	delete(*eventFields, "timestamp")
	event := beat.Event{
		Timestamp: ts,
		Fields:    *eventFields,
	}
	bt.client.Publish(event)
}

func parseMacTimestamp(timestamp string) (time.Time, error) {
	macTimestampFormat := "2006-01-02 15:04:05.000000Z0700"
	t, err := time.Parse(macTimestampFormat, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("Timestamp conversion failed for '%s'", timestamp)
	}
	return t, nil
}

func publishLogsSince(startTime time.Time, bt *macoslogbeat) error {
	layout := "2006-01-02 15:04:05Z0700"
	args := append(buildArgs("show", bt.config.ExcludedSubsystems),
		"--start", startTime.Format(layout),
		"--end", time.Now().Format(layout))
	cmd := exec.Command("/usr/bin/log", args...)
	stdout, _ := cmd.StdoutPipe()
	err := cmd.Start()

	decoder := jstream.NewDecoder(bufio.NewReader(stdout), 1)
	for mv := range decoder.Stream() {
		f := common.MapStr(mv.Value.(map[string]interface{}))
		// When viewing old logs creatorActivityID gets some really crazy values that are not indexable
		f["creatorActivityID"] = float64(0)
		publishEvent(bt, &f)
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("reading old logs did not exit succesfully exited with code %d", exitError.ExitCode())
		}
	}

	return nil
}

// Run starts macoslogbeat.
func (bt *macoslogbeat) Run(b *beat.Beat) error {
	logp.Info("macoslogbeat is running! Hit CTRL-C to stop it.")
	var err error
	var counter int
	timestampFile := "timestamp"
	app := "/usr/bin/log"
	cmd := exec.Command(app, buildArgs("stream", bt.config.ExcludedSubsystems)...)

	stdout, err := cmd.StdoutPipe()
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)

	cmd.Start()
	reader := bufio.NewReader(stdout)
	if len(bt.config.ExcludedSubsystems) > 0 {
		// Read the first line from buffer because it's not json when predicates are applied
		reader.ReadBytes('\n')
	}
	dec := jstream.NewDecoder(reader, 1)

	lastPublishedLog, err := readTimestamp(path.Join(bt.config.CacheDir, timestampFile))
	if err == nil {
		logp.Info("Shipping old logs since %s", lastPublishedLog.Format(rfc3339ms))
		if err = publishLogsSince(lastPublishedLog, bt); err != nil {
			logp.Err(err.Error())
		} else {
			logp.Info("Old logs shipped")
		}
	}

	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		for mv := range dec.Stream() {
			fields := common.MapStr(mv.Value.(map[string]interface{}))
			eventTs, _ := parseMacTimestamp(fields["timestamp"].(string))
			publishEvent(bt, &fields)
			counter++
			if counter%100 == 0 {
				writeTimestamp(eventTs, path.Join(bt.config.CacheDir, timestampFile))
			}
		}
	}
}

// Stop stops macoslogbeat.
func (bt *macoslogbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
