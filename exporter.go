/*
 * Copyright (c) 2021 Huy Duc Dao
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package opencensus_influxdb_exporter

import (
	"context"
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
	"go.opencensus.io/stats/view"
	"log"
	"sync"
	"time"
)

const (
	defaultBufferSize      = 1000 * 1000
	defaultReportingPeriod = 15 * time.Second
)

type Options struct {
	Address         string
	Username        string
	Password        string
	Timeout         time.Duration
	PingEnabled     bool
	Database        string
	InstanceName    string
	BufferSize      int
	ReportingPeriod time.Duration
	OnError         func(err error)
}

func NewExporter(ctx context.Context, o Options) (*IFExporter, error) {
	c, err := newClient(o)
	if err != nil {
		return nil, err
	}

	onError := func(err error) {
		if o.OnError != nil {
			o.OnError(err)
			return
		}
		log.Printf("Error when uploading stats to Influx: %s", err.Error())
	}

	e := &IFExporter{
		opts:    o,
		client:  c,
		buffer:  newBuffer(o.BufferSize),
		onError: onError,
	}

	reportingPeriod := o.ReportingPeriod
	if reportingPeriod <= 0 {
		reportingPeriod = defaultReportingPeriod
	}

	go e.flushBuffer(ctx, time.NewTicker(reportingPeriod))

	return e, nil
}

type IFExporter struct {
	opts    Options
	client  Client
	buffer  *buffer
	onError func(error)
}

var _ view.Exporter = (*IFExporter)(nil)

func (e *IFExporter) ExportView(vd *view.Data) {
	if len(vd.Rows) == 0 {
		return
	}

	bp := e.batch()
	bp.AddPoints(e.viewToPoints(vd))

	e.buffer.Add(bp)
}

func (e *IFExporter) flushBuffer(ctx context.Context, ticker *time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bps := e.buffer.Elements()
			if len(bps) == 0 {
				continue
			}
			bp := e.batch()
			for _, b := range bps {
				bp.AddPoints(b.Points())
			}
			if err := e.client.Write(bp); err != nil {
				e.buffer.Add(bps...)
				e.onError(exportError{err, bp})
			}
		}
	}
}

func (e *IFExporter) batch() client.BatchPoints {
	bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  e.opts.Database,
		Precision: "s",
	})
	return bp
}

func (e *IFExporter) viewToPoints(vd *view.Data) []*client.Point {
	var pts []*client.Point
	for _, row := range vd.Rows {
		f, err := e.toFields(row)
		if err != nil {
			continue
		}
		p, err := client.NewPoint(vd.View.Name, e.toTags(row), f, vd.End)
		if err != nil {
			continue
		}
		pts = append(pts, p)
		data, ok := row.Data.(*view.DistributionData)
		if !ok {
			continue
		}
		pts = append(pts, e.parseBuckets(vd.View, data, vd.End)...)
	}
	return pts
}

func (e *IFExporter) toTags(row *view.Row) map[string]string {
	res := make(map[string]string, len(row.Tags)+1)
	for _, tag := range row.Tags {
		res[tag.Key.Name()] = tag.Value
	}
	res["instance"] = e.opts.InstanceName
	return res
}

func (e *IFExporter) toFields(row *view.Row) (map[string]interface{}, error) {
	switch data := row.Data.(type) {
	case *view.CountData:
		return map[string]interface{}{"count": data.Value}, nil
	case *view.SumData:
		return map[string]interface{}{"sum": data.Value}, nil
	case *view.LastValueData:
		return map[string]interface{}{"last": data.Value}, nil
	case *view.DistributionData:
		return map[string]interface{}{
			"sum":   data.Sum(),
			"count": data.Count,
			"max":   data.Max,
			"min":   data.Min,
			"mean":  data.Mean,
		}, nil
	default:
		err := fmt.Errorf("aggregation %T is not yet supported", data)
		log.Print(err)
		return map[string]interface{}{}, err
	}
}

func (e *IFExporter) parseBuckets(v *view.View, data *view.DistributionData, timestamp time.Time) []*client.Point {
	var res []*client.Point

	indicesMap := make(map[float64]int)
	buckets := make([]float64, 0, len(v.Aggregation.Buckets))
	for i, b := range v.Aggregation.Buckets {
		if _, ok := indicesMap[b]; !ok {
			indicesMap[b] = i
			buckets = append(buckets, b)
		}
	}
	for _, b := range buckets {
		if value := data.CountPerBucket[indicesMap[b]]; value != 0 {
			pt, err := client.NewPoint(
				v.Name+"_buckets",
				map[string]string{
					"bucket":   fmt.Sprintf("%.0f", b),
					"instance": e.opts.InstanceName,
				},
				map[string]interface{}{"count": value},
				timestamp,
			)
			if err != nil {
				continue
			}
			res = append(res, pt)
		}
	}

	return res
}

type Client interface {
	Ping(timeout time.Duration) (time.Duration, string, error)
	Write(bp client.BatchPoints) error
}

func newClient(o Options) (client.Client, error) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     o.Address,
		Username: o.Username,
		Password: o.Password,
		Timeout:  o.Timeout,
	})

	if err != nil {
		return nil, err
	}

	if o.PingEnabled {
		if _, _, err := c.Ping(time.Second); err != nil {
			return nil, err
		}
	}

	return c, nil
}

type exportError struct {
	err error
	bp  client.BatchPoints
}

func (e exportError) Error() string {
	return e.err.Error()
}

func newBuffer(size int) *buffer {
	if size == 0 {
		size = defaultBufferSize
	}
	return &buffer{
		data: []client.BatchPoints{},
		size: size,
		mu:   new(sync.Mutex),
	}
}

type buffer struct {
	data []client.BatchPoints
	size int
	mu   *sync.Mutex
}

func (b *buffer) Add(ps ...client.BatchPoints) {
	b.mu.Lock()
	b.data = append(b.data, ps...)
	if len(b.data) > b.size {
		b.data = b.data[len(b.data)-b.size:]
	}
	b.mu.Unlock()
}

func (b *buffer) Elements() []client.BatchPoints {
	var res []client.BatchPoints
	b.mu.Lock()
	res, b.data = b.data, []client.BatchPoints{}
	b.mu.Unlock()
	return res
}
