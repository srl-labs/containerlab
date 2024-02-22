// Copyright 2024 Yutaro Hayakawa
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package k8s_kind

import (
	log "github.com/sirupsen/logrus"
	kindLog "sigs.k8s.io/kind/pkg/log"
)

// kindLogger implements the log.Logger interface for kind.
type kindLogger struct {
	l log.FieldLogger
	v kindLog.Level
}

func newKindLogger(clusterName string, v kindLog.Level) *kindLogger {
	return &kindLogger{
		l: log.WithField("kind-cluster", clusterName),
		v: v,
	}
}

func (l *kindLogger) Warn(message string) {
	l.l.Warn(message)
}

func (l *kindLogger) Warnf(format string, args ...interface{}) {
	l.l.Warnf(format, args...)
}

func (l *kindLogger) Error(message string) {
	l.l.Error(message)
}

func (l *kindLogger) Errorf(format string, args ...interface{}) {
	l.l.Errorf(format, args...)
}

func (l *kindLogger) V(v kindLog.Level) kindLog.InfoLogger {
	return &kindInfoLogger{
		l:       l.l,
		v:       v,
		enabled: v <= l.v,
	}
}

type kindInfoLogger struct {
	l       log.FieldLogger
	v       kindLog.Level
	enabled bool
}

func (l *kindInfoLogger) Info(message string) {
	if !l.enabled {
		return
	}
	l.l.Info(message)
}

func (l *kindInfoLogger) Infof(format string, args ...interface{}) {
	if !l.enabled {
		return
	}
	l.l.Infof(format, args...)
}

func (l *kindInfoLogger) Enabled() bool {
	return l.enabled
}
