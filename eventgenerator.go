package main

import (
	"github.com/elastic/libbeat/common"
	"github.com/fsouza/go-dockerclient"
	"time"
)

type EventGenerator struct {
	networkStats map[string]NetworkData
}

func (d *EventGenerator) getContainerEvent(container *docker.APIContainers, stats *docker.Stats) common.MapStr {
	event := common.MapStr{
		"timestamp":      common.Time(stats.Read),
		"type":           "container",
		"containerID":    container.ID,
		"containerNames": container.Names,
		"container": common.MapStr{
			"id":         container.ID,
			"command":    container.Command,
			"created":    time.Unix(container.Created, 0),
			"image":      container.Image,
			"labels":     container.Labels,
			"names":      container.Names,
			"ports":      d.convertContainerPorts(&container.Ports),
			"sizeRootFs": container.SizeRootFs,
			"sizeRw":     container.SizeRw,
			"status":     container.Status,
		},
	}
	return event
}

func (d *EventGenerator) getCpuEvent(container *docker.APIContainers, stats *docker.Stats) common.MapStr {

	calculator := CPUCalculator{
		CPUData{stats.PreCPUStats.CPUUsage.PercpuUsage, stats.PreCPUStats.CPUUsage.TotalUsage, stats.PreCPUStats.CPUUsage.UsageInKernelmode, stats.PreCPUStats.CPUUsage.UsageInUsermode},
		CPUData{stats.CPUStats.CPUUsage.PercpuUsage, stats.CPUStats.CPUUsage.TotalUsage, stats.CPUStats.CPUUsage.UsageInKernelmode, stats.CPUStats.CPUUsage.UsageInUsermode},
	}

	event := common.MapStr{
		"timestamp":      common.Time(stats.Read),
		"type":           "cpu",
		"containerID":    container.ID,
		"containerNames": container.Names,
		"cpu": common.MapStr{
			"percpuUsage":       calculator.perCpuUsage(),
			"totalUsage":        calculator.totalUsage(),
			"usageInKernelmode": calculator.usageInKernelmode(),
			"usageInUsermode":   calculator.usageInUsermode(),
		},
	}

	return event
}

func (d *EventGenerator) getNetworkEvent(container *docker.APIContainers, stats *docker.Stats) common.MapStr {
	newNetworkData := NetworkData{
		stats.Read,
		stats.Network.RxBytes,
		stats.Network.RxDropped,
		stats.Network.RxErrors,
		stats.Network.RxPackets,
		stats.Network.TxBytes,
		stats.Network.TxDropped,
		stats.Network.TxErrors,
		stats.Network.TxPackets,
	}

	var event common.MapStr

	oldNetworkData, ok := d.networkStats[container.ID]

	if ok {
		calculator := NetworkCalculator{oldNetworkData, newNetworkData}
		event = common.MapStr{
			"timestamp":      common.Time(stats.Read),
			"type":           "net",
			"containerID":    container.ID,
			"containerNames": container.Names,
			"net": common.MapStr{
				"rxBytes_ps":   calculator.getRxBytesPerSecond(),
				"rxDropped_ps": calculator.getRxDroppedPerSecond(),
				"rxErrors_ps":  calculator.getRxErrorsPerSecond(),
				"rxPackets_ps": calculator.getRxPacketsPerSecond(),
				"txBytes_ps":   calculator.getTxBytesPerSecond(),
				"txDropped_ps": calculator.getTxDroppedPerSecond(),
				"txErrors_ps":  calculator.getTxErrorsPerSecond(),
				"txPackets_ps": calculator.getTxPacketsPerSecond(),
			},
		}
	} else {
		event = common.MapStr{
			"timestamp":      common.Time(stats.Read),
			"type":           "net",
			"containerID":    container.ID,
			"containerNames": container.Names,
			"net": common.MapStr{
				"rxBytes":   0,
				"rxDropped": 0,
				"rxErrors":  0,
				"rxPackets": 0,
				"txBytes":   0,
				"txDropped": 0,
				"txErrors":  0,
				"txPackets": 0,
			},
		}
	}

	d.networkStats[container.ID] = newNetworkData
	return event
}

func (d *EventGenerator) getMemoryEvent(container *docker.APIContainers, stats *docker.Stats) common.MapStr {
	event := common.MapStr{
		"timestamp":      common.Time(stats.Read),
		"type":           "memory",
		"containerID":    container.ID,
		"containerNames": container.Names,
		"memory": common.MapStr{
			"failcnt":  stats.MemoryStats.Failcnt,
			"limit":    stats.MemoryStats.Limit,
			"maxUsage": stats.MemoryStats.MaxUsage,
			"usage":    stats.MemoryStats.Usage,
			"usage_p":  (float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit)) * 100,
		},
	}

	return event
}

func (d *EventGenerator) convertContainerPorts(ports *[]docker.APIPort) []map[string]interface{} {
	var outputPorts []map[string]interface{}
	for _, port := range *ports {
		outputPort := common.MapStr{
			"ip":          port.IP,
			"privatePort": port.PrivatePort,
			"publicPort":  port.PublicPort,
			"type":        port.Type,
		}
		outputPorts = append(outputPorts, outputPort)
	}

	return outputPorts
}

func (d *EventGenerator) cleanOldStats(containers []docker.APIContainers) {
	found := false
	for containerStatKey, _ := range d.networkStats {
		for _, container := range containers {
			if container.ID == containerStatKey {
				found = true
				continue
			}
		}
		if !found {
			delete(d.networkStats, containerStatKey)
		}
	}
}
