package beater

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
		logp.Err("Could not update cache file timestamp: %v", err)
	}
}

func readTimestamp(fileName string) (time.Time, error) {
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not read contents of %s: %w", fileName, err)
	}
	t, err := time.Parse(rfc3339ms, string(contents))
	if err != nil {
		return time.Time{}, fmt.Errorf("Timestamp is malformed: %w", err)
	}
	return t, nil
}

func publishEvent(bt *macoslogbeat, eventFields common.MapStr) {
	ts, err := parseMacTimestamp(eventFields["timestamp"].(string))
	if err != nil {
		logp.Err("Cant use event timestamp, using time.Now(): %v", err)
		ts = time.Now()
	}
	delete(eventFields, "timestamp")
	if len(eventFields["eventMessage"].(string)) == 0 {
		return
	}
	event := beat.Event{
		Timestamp: ts,
		Fields:    common.MapStr{"unifiedlog": eventFields},
	}
	bt.client.Publish(event)
}

func parseMacTimestamp(timestamp string) (time.Time, error) {
	macTimestampFormat := "2006-01-02 15:04:05.000000Z0700"
	t, err := time.Parse(macTimestampFormat, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("Timestamp conversion failed for '%s': %w", timestamp, err)
	}
	return t, nil
}

func readLogWithArgs(commandArgs []string, skipLines int) (<-chan common.MapStr, error) {
	var err error
	cmd := exec.Command("/usr/bin/log", commandArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("attaching pipe to commands stdout failed: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("starting command failed: %w", err)
	}

	reader := bufio.NewReader(stdout)

	for i := 0; i < skipLines; i++ {
		_, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("Reading from buffer failed: %w", err)
		}
	}

	decoder := jstream.NewDecoder(reader, 1)
	c := make(chan common.MapStr)
	go func() {
		for mv := range decoder.Stream() {
			c <- common.MapStr(mv.Value.(map[string]interface{}))
		}
		close(c)
		io.Copy(ioutil.Discard, stdout)
		cmd.Wait()
	}()
	return c, nil
}

func publishOldLogs(bt *macoslogbeat, timeStampFile string) error {
	var err error
	if _, err := os.Stat(timeStampFile); err != nil {
		logp.Info("Timestamp does not exist, this is probably the first run.")
		return nil
	}

	lastPublish, err := readTimestamp(timeStampFile)
	if err != nil {
		return fmt.Errorf("could not read timestamp: %w", err)
	}

	layout := "2006-01-02 15:04:05Z0700"
	args := append(buildArgs("show", bt.config.ExcludedSubsystems),
		"--start", lastPublish.Format(layout),
		"--end", time.Now().Format(layout))

	logp.Info("Publishing old logs since %s", lastPublish.Format(layout))
	logStream, err := readLogWithArgs(args, 0)
	if err != nil {
		return fmt.Errorf("could read not old logs %w", err)
	}
	for fields := range logStream {
		// Set creatorActivityID because it gets crazy and non-indexable values when using show instead of stream
		fields["creatorActivityID"] = float64(0)
		publishEvent(bt, fields)
	}
	logp.Info("Old logs published")

	return nil
}

// Run starts macoslogbeat.
func (bt *macoslogbeat) Run(b *beat.Beat) error {
	logp.Info("macoslogbeat is running! Hit CTRL-C to stop it.")
	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	args := buildArgs("stream", bt.config.ExcludedSubsystems)
	var skipLines int = 0
	if len(bt.config.ExcludedSubsystems) > 0 {
		skipLines = 1
	}
	logStream, err := readLogWithArgs(args, skipLines)
	if err != nil {
		return err
	}

	timestampFile := path.Join(bt.config.CacheDir, "timestamp")
	if err = publishOldLogs(bt, timestampFile); err != nil {
		logp.Err("Problems when publishing old logs: %v", err)

	}

	ticker := time.NewTicker(bt.config.Period)
	var counter int
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:

		case event := <-logStream:
			eventTs, _ := parseMacTimestamp(event["timestamp"].(string))
			publishEvent(bt, event)
			counter++
			if counter%100 == 0 {
				writeTimestamp(eventTs, timestampFile)
			}
		}
	}
}

// Stop stops macoslogbeat.
func (bt *macoslogbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
