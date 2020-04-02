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

func readLogWithArgs(commandArgs []string, skipLines int) (chan common.MapStr, error) {
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

	if skipLines > 0 {
		for i := 0; i < skipLines; i++ {
			reader.ReadBytes('\n')
		}
	}

	decoder := jstream.NewDecoder(reader, 1)
	c := make(chan common.MapStr)
	go func() {
		for mv := range decoder.Stream() {
			c <- common.MapStr(mv.Value.(map[string]interface{}))
		}
		close(c)
		cmd.Wait()
	}()
	return c, nil
}

func publishLogsSince(bt *macoslogbeat, startTime time.Time) error {
	var err error
	layout := "2006-01-02 15:04:05Z0700"
	args := append(buildArgs("show", bt.config.ExcludedSubsystems),
		"--start", startTime.Format(layout),
		"--end", time.Now().Format(layout))

	logStream, err := readLogWithArgs(args, 0)
	if err != nil {
		return fmt.Errorf("could not old logs %w", err)
	}
	for fields := range logStream {
		fields["creatorActivityID"] = float64(0)
		publishEvent(bt, fields)
	}

	return nil
}

// Run starts macoslogbeat.
func (bt *macoslogbeat) Run(b *beat.Beat) error {
	logp.Info("macoslogbeat is running! Hit CTRL-C to stop it.")
	var err error
	var counter int
	timestampFile := "timestamp"
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	lastPublishedLog, err := readTimestamp(path.Join(bt.config.CacheDir, timestampFile))
	if err == nil {
		logp.Info("Shipping old logs since %s", lastPublishedLog.Format(rfc3339ms))
		if err = publishLogsSince(bt, lastPublishedLog); err != nil {
			logp.Err(err.Error())
		} else {
			logp.Info("Old logs shipped")
		}
	}

	args := buildArgs("stream", bt.config.ExcludedSubsystems)
	logStream, err := readLogWithArgs(args, 1)
	for event := range logStream {
		eventTs, _ := parseMacTimestamp(event["timestamp"].(string))
		publishEvent(bt, event)
		counter++
		if counter%100 == 0 {
			writeTimestamp(eventTs, path.Join(bt.config.CacheDir, timestampFile))
		}
	}
	return nil
}

// Stop stops macoslogbeat.
func (bt *macoslogbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
