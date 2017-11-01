package ggsci

import (
	"fmt"
	"github.com/intelsdi-x/snap-plugin-lib-go/v1/plugin"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	PluginName    = "ggsci"
	PluginVersion = 1
	PluginVedor   = "mfms"
	ggsciPath     = "ggsci_path"
)

type Plugin struct {
	initialized bool
	ggsciPath   string
}

type parseResult struct {
	component string
	name      string
	state     string
	lag       int
}

func NewCollector() *Plugin {
	return &Plugin{initialized: false}
}

func (p *Plugin) GetConfigPolicy() (plugin.ConfigPolicy, error) {
	policy := plugin.NewConfigPolicy()
	policy.AddNewStringRule([]string{PluginVedor, PluginName}, ggsciPath, true)
	return *policy, nil
}

func (p *Plugin) GetMetricTypes(plugin.Config) ([]plugin.Metric, error) {
	mts := []plugin.Metric{}
	namespace := createNamespace("state")
	mts = append(mts, plugin.Metric{Namespace: namespace})

	namespace = createNamespace("lag")
	mts = append(mts, plugin.Metric{Namespace: namespace})

	return mts, nil
}

func (p *Plugin) CollectMetrics(metrics []plugin.Metric) ([]plugin.Metric, error) {
	var err error
	mts := []plugin.Metric{}
	results := []parseResult{}

	if !p.initialized {
		p.ggsciPath, err = metrics[0].Config.GetString(ggsciPath)
		if err != nil {
			return nil, err
		}
		p.initialized = true
	}

	cmd := exec.Command("sudo", "LD_LIBRARY_PATH="+os.Getenv("LD_LIBRARY_PATH"), p.ggsciPath)
	cmdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	io.WriteString(cmdin, "info all\nexit\n")
	cmdin.Close()
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	outlines := strings.Split(string(output), "\n")
	idx := getValueIndexRegex(outlines, "Program\\s+Status\\s+Group\\s+Lag at Chkpt\\s+Time Since Chkpt")
	if idx < 0 {
		return nil, fmt.Errorf("no data to process")
	}

	lines := outlines[idx+2 : len(outlines)-3]

	for _, line := range lines {
		result := parseResult{}
		fields := strings.Fields(line)
		if len(fields) == 2 {
			result.component = fields[0]
			result.state = fields[1]
			result.name = fields[0]
		}
		if len(fields) == 5 {
			result.component = fields[0]
			result.state = fields[1]
			result.name = fields[2]
			lag1, err := processLagTime(fields[3])
			lag2, err := processLagTime(fields[4])
			if err != nil {
				return nil, err
			}

			result.lag = lag1 + lag2
		}
		results = append(results, result)
	}

	ts := time.Now()
	for _, metric := range metrics {
		switch metric.Namespace[len(metric.Namespace)-1].Value {
		case "state":
			for _, result := range results {
				mt := plugin.Metric{
					Namespace: createNamespace("state"),
					Timestamp: ts,
				}
				mt.Namespace[2].Value = result.component
				mt.Namespace[3].Value = result.name
				if mt.Data = 1; result.state == "RUNNING" {
					mt.Data = 0
				}
				mts = append(mts, mt)
			}
		case "lag":
			for _, result := range results {
				if result.component == "MANAGER" {
					continue
				}
				mt := plugin.Metric{
					Namespace: createNamespace("lag"),
					Timestamp: ts,
				}
				mt.Namespace[2].Value = result.component
				mt.Namespace[3].Value = result.name
				mt.Data = result.lag
				mts = append(mts, mt)
			}
		}
	}

	return mts, nil
}

func getValueIndexRegex(arr []string, regex string) int {
	re := regexp.MustCompile(regex)
	for idx, v := range arr {
		if r := re.FindStringIndex(v); len(r) > 0 {
			return idx
		}
	}
	return -1
}

func processLagTime(lag string) (int, error) {
	parts := strings.Split(lag, ":")
	if len(parts) < 3 {
		return 0, fmt.Errorf("wrong time lag format, value: %v", lag)
	}
	h, err := strconv.Atoi(parts[0])
	m, err := strconv.Atoi(parts[1])
	s, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}
	return s + m*60 + h*60*60, nil
}

func createNamespace(lastelement string) plugin.Namespace {
	namespace := plugin.NewNamespace(PluginVedor, PluginName)
	namespace = namespace.AddDynamicElement("component", "component type")
	namespace = namespace.AddDynamicElement("name", "component name")
	namespace = namespace.AddStaticElement(lastelement)
	return namespace
}
