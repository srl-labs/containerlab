package clabernetes

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Runtime) pollInterfaceStats(
	ctx context.Context,
	namespace string,
	interval time.Duration,
	eventSink chan<- clablabruntime.Event,
) {
	if interval <= 0 {
		interval = time.Second
	}

	samples := map[string]c9sIfaceStatsSample{}

	sample := func() {
		states, err := r.List(ctx, clablabruntime.ListRequest{
			Namespace:     namespace,
			AllNamespaces: namespace == metav1.NamespaceAll,
		})
		if err != nil {
			log.Debug("failed to list clabernetes topologies for interface stats", "error", err)
			return
		}

		now := time.Now()
		for _, state := range states {
			for _, node := range state.Nodes {
				if !node.Ready {
					continue
				}

				pod, err := r.launcherPod(ctx, state.Name, state.Namespace, node.Name)
				if err != nil {
					log.Debug("failed to resolve clabernetes launcher pod for interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"error", err,
					)
					continue
				}

				stdout, stderr, rc, err := r.execInPod(ctx, pod,
					[]string{"docker", "exec", node.Name, "cat", "/proc/net/dev"})
				if err != nil {
					log.Debug("failed to collect clabernetes interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"error", err,
					)
					continue
				}
				if rc != 0 {
					log.Debug("failed to collect clabernetes interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"rc", rc,
						"stderr", strings.TrimSpace(string(stderr)),
					)
					continue
				}

				stats, err := parseProcNetDev(stdout)
				if err != nil {
					log.Debug("failed to parse clabernetes interface stats",
						"namespace", state.Namespace,
						"lab", state.Name,
						"node", node.Name,
						"error", err,
					)
					continue
				}

				for _, stat := range stats {
					key := c9sIfaceStatsKey(state.Namespace, state.Name, node.Name, stat.Name)
					current := c9sIfaceStatsSample{
						Stats:     stat,
						Timestamp: now,
					}

					if previous, ok := samples[key]; ok {
						event := c9sIfaceStatsEvent(state, node, pod, stat, previous, current)
						r.sendEvent(ctx, eventSink, event)
					}

					samples[key] = current
				}
			}
		}
	}

	sample()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sample()
		}
	}
}

type c9sIfaceStats struct {
	Name      string
	RxBytes   uint64
	RxPackets uint64
	TxBytes   uint64
	TxPackets uint64
}

type c9sIfaceStatsSample struct {
	Stats     c9sIfaceStats
	Timestamp time.Time
}

func parseProcNetDev(data []byte) ([]c9sIfaceStats, error) {
	lines := strings.Split(string(data), "\n")
	stats := make([]c9sIfaceStats, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		ifName := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			return nil, fmt.Errorf("unexpected /proc/net/dev line for %q: %q", ifName, line)
		}

		rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rx bytes for %q: %w", ifName, err)
		}
		rxPackets, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rx packets for %q: %w", ifName, err)
		}
		txBytes, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx bytes for %q: %w", ifName, err)
		}
		txPackets, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx packets for %q: %w", ifName, err)
		}

		stats = append(stats, c9sIfaceStats{
			Name:      ifName,
			RxBytes:   rxBytes,
			RxPackets: rxPackets,
			TxBytes:   txBytes,
			TxPackets: txPackets,
		})
	}

	return stats, nil
}

func c9sIfaceStatsKey(namespace, lab, node, ifName string) string {
	return namespace + "/" + lab + "/" + node + "/" + ifName
}

func c9sIfaceStatsEvent(
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	pod *corev1.Pod,
	stat c9sIfaceStats,
	previous,
	current c9sIfaceStatsSample,
) clablabruntime.Event {
	interval := current.Timestamp.Sub(previous.Timestamp)
	if interval <= 0 {
		interval = time.Second
	}

	seconds := interval.Seconds()
	rxBytesDelta := counterDelta(stat.RxBytes, previous.Stats.RxBytes)
	txBytesDelta := counterDelta(stat.TxBytes, previous.Stats.TxBytes)
	rxPacketsDelta := counterDelta(stat.RxPackets, previous.Stats.RxPackets)
	txPacketsDelta := counterDelta(stat.TxPackets, previous.Stats.TxPackets)

	actorName := fmt.Sprintf("%s-%s", state.Name, node.Name)
	podName := ""
	if pod != nil {
		podName = pod.Name
	}

	return clablabruntime.Event{
		Timestamp:   current.Timestamp,
		Type:        "interface",
		Action:      "stats",
		ActorID:     c9sIfaceStatsKey(state.Namespace, state.Name, node.Name, stat.Name),
		ActorName:   actorName,
		ActorFullID: podName,
		Attributes: map[string]string{
			"namespace":        state.Namespace,
			"lab":              state.Name,
			"node":             node.Name,
			"name":             actorName,
			"pod":              podName,
			"ifname":           stat.Name,
			"origin":           "clabernetes",
			"rx_bytes":         strconv.FormatUint(stat.RxBytes, 10),
			"tx_bytes":         strconv.FormatUint(stat.TxBytes, 10),
			"rx_packets":       strconv.FormatUint(stat.RxPackets, 10),
			"tx_packets":       strconv.FormatUint(stat.TxPackets, 10),
			"rx_bps":           strconv.FormatFloat(float64(rxBytesDelta*8)/seconds, 'f', -1, 64),
			"tx_bps":           strconv.FormatFloat(float64(txBytesDelta*8)/seconds, 'f', -1, 64),
			"rx_pps":           strconv.FormatFloat(float64(rxPacketsDelta)/seconds, 'f', -1, 64),
			"tx_pps":           strconv.FormatFloat(float64(txPacketsDelta)/seconds, 'f', -1, 64),
			"interval_seconds": strconv.FormatFloat(seconds, 'f', -1, 64),
		},
	}
}

func counterDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}

	return current - previous
}
