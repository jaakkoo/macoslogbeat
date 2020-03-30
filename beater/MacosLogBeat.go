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

func writeTimestamp(timestamp string, fileName string) {
	err := ioutil.WriteFile(fileName, []byte(timestamp), 0600)
	if err != nil {
		logp.Err("Could not update cache file timestamp")
	}
}

func readTimestamp(fileName string) (timestamp time.Time, err error) {
	layout := "2006-01-02 15:04:05.000000Z0700"
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not read contents of %s", fileName)
	}
	t, err := time.Parse(layout, string(contents))
	if err != nil {
		return time.Time{}, fmt.Errorf("Timestamp is malformed")
	}
	return t, nil
}

func publishEvent(bt *macoslogbeat, eventFields *common.MapStr) {
	event := beat.Event{
		Timestamp: time.Now(),
		Fields:    *eventFields,
	}
	bt.client.Publish(event)
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
		logp.Info("Shipping old logs since %s", lastPublishedLog)
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
			publishEvent(bt, &fields)
			counter++
			if counter%100 == 0 {
				writeTimestamp(fields["timestamp"].(string), path.Join(bt.config.CacheDir, timestampFile))
			}
		}
	}
}

// Stop stops macoslogbeat.
func (bt *macoslogbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
