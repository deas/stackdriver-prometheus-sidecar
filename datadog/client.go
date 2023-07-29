// Copyright 2016 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	// monitoring "google.golang.org/genproto/googleapis/monitoring/v3"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	// "google.golang.org/grpc/balancer/roundrobin"

	"github.com/Stackdriver/stackdriver-prometheus-sidecar/stackdriver"
)

/*
const (
	MaxTimeseriesesPerRequest = 200
	MonitoringWriteScope      = "https://www.googleapis.com/auth/monitoring.write"
)

var (
	// StatusTag is the google3 canonical status code: google3/google/rpc/code.proto
	StatusTag = tag.MustNewKey("status")

	// PointCount is a metric.
	PointCount = stats.Int64("agent.googleapis.com/agent/monitoring/point_count",
		"count of metric points written to Stackdriver", stats.UnitDimensionless)
)
*/

func init() {
	/*
		if err := view.Register(
			&view.View{
				Measure:     stackdriver.PointCount,
				TagKeys:     []tag.Key{stackdriver.StatusTag},
				Aggregation: view.Sum(),
			},
		); err != nil {
			panic(err)
		}
	*/
}

// Client allows reading and writing from/to a remote gRPC endpoint. The
// implementation may hit a single backend, so the application should create a
// number of these clients.
type Client struct {
	datadog.APIClient
	// datadog.NewAPIClient(configuration)
	logger log.Logger
	// projectID string
	// url     *url.URL
	timeout time.Duration
	// resolver  *manual.Resolver
	// conn *grpc.ClientConn
}

// ClientConfig configures a Client.
/*
type ClientConfig struct {
	Logger log.Logger
	// ProjectID string // The Stackdriver project ID in "projects/name-or-number" format.
	URL     *url.URL
	Timeout time.Duration
	// Resolver  *manual.Resolver
}
*/

// NewClient creates a new Client.
func NewClient(cfg *datadog.Configuration /**ClientConfig*/, timeout time.Duration, logger log.Logger) *Client {
	// logger := conf.Logger
	if logger == nil {
		logger = log.NewNopLogger()
	}
	/*
		if cfg.HTTPClient == nil {
			cfg.HTTPClient = http.DefaultClient
		}
	*/
	// apiClient := datadog.NewAPIClient(cfg)

	return &Client{*datadog.NewAPIClient(cfg), logger, timeout}
	/*
		c := &Client{
			// APIClient: apiClient,
			logger: logger,
			// projectID: conf.ProjectID,
			// url:     conf.URL,
			timeout: timeout, //conf.Timeout,
			// resolver:  conf.Resolver,
		}
		c.Cfg = cfg

		return c
	*/
}

type recoverableError struct {
	error
}

// version.* is populated for 'promu' builds, so this will look broken in unit tests.
// var userAgent = fmt.Sprintf("StackdriverPrometheus/%s", version.Version)
/*
func (c *Client) getConnection(ctx context.Context) (*grpc.ClientConn, error) {
		if c.conn != nil {
			return c.conn, nil
		}

		useAuth, err := strconv.ParseBool(c.url.Query().Get("auth"))
		if err != nil {
			useAuth = true // Default to auth enabled.
		}
		level.Debug(c.logger).Log(
			"msg", "is auth enabled",
			"auth", useAuth,
			"url", c.url.String())
		// Google APIs currently return a single IP for the whole service.  gRPC
		// client-side load-balancing won't spread the load across backends
		// while that's true, but it also doesn't hurt.
		dopts := []grpc.DialOption{
			// grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
			// grpc.WithBalancerName(roundrobin.Name), // Is default
			grpc.WithBlock(), // Wait for the connection to be established before using it.
			// grpc.WithUserAgent(userAgent),
			grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		}
		if useAuth {
			rpcCreds, err := oauth.NewApplicationDefault(context.Background(), stackdriver.MonitoringWriteScope)
			if err != nil {
				return nil, err
			}
			tlsCreds := credentials.NewTLS(&tls.Config{})
			dopts = append(dopts,
				grpc.WithTransportCredentials(tlsCreds),
				grpc.WithPerRPCCredentials(rpcCreds))
		} else {
			dopts = append(dopts, grpc.WithInsecure())
		}
		address := c.url.Hostname()
		if len(c.url.Port()) > 0 {
			address = net.JoinHostPort(address, c.url.Port())
		}
		if c.resolver != nil {
			address = c.resolver.Scheme() + ":///" + address
		}
		conn, err := grpc.DialContext(ctx, address, dopts...)
		c.conn = conn
		if err == context.DeadlineExceeded {
			return conn, recoverableError{err}
		}
		return conn, err
	return nil, nil
}
*/

// Store sends a batch of samples to the HTTP endpoint.
func (c *Client) Store(tss []datadogV2.MetricSeries /*monitoring.CreateTimeSeriesRequest*/) error {
	// tss := req.TimeSeries
	if len(tss) == 0 {
		// Nothing to do, return silently.
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	/* conn, err := c.getConnection(ctx)
	if err != nil {
		return err
	}
	service := monitoring.NewMetricServiceClient(conn)
	*/
	service := datadogV2.NewMetricsApi(&c.APIClient)

	errors := make(chan error, len(tss)/stackdriver.MaxTimeseriesesPerRequest+1)
	var wg sync.WaitGroup
	for i := 0; i < len(tss); i += stackdriver.MaxTimeseriesesPerRequest {
		end := i + stackdriver.MaxTimeseriesesPerRequest
		if end > len(tss) {
			end = len(tss)
		}
		wg.Add(1)
		go func(begin int, end int) {
			defer wg.Done()
			req_copy := tss[begin:end]
			/*&monitoring.CreateTimeSeriesRequest{
				Name:       c.projectID,
				TimeSeries: req.TimeSeries[begin:end],
			}*/
			body := datadogV2.MetricPayload{
				Series: req_copy,
			}

			level.Debug(c.logger).Log(
				"msg", "Partial failure calling MetricPayload",
				// "err", err
			)
			_, _, err := service.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())
			if err == nil {
				// The response is empty if all points were successfully written.
				stats.RecordWithTags(ctx,
					[]tag.Mutator{tag.Upsert(stackdriver.StatusTag, "0")},
					stackdriver.PointCount.M(int64(end-begin)))
			} else {
				level.Debug(c.logger).Log(
					"msg", "Partial failure calling MetricPayload",
					"err", err)
				status, ok := status.FromError(err)
				if !ok {
					level.Warn(c.logger).Log("msg", "Unexpected error message type from Metrics API", "err", err)
					errors <- err
					return
				}
				for _, details := range status.Details() {
					if summary, ok := details.(*monitoring.CreateTimeSeriesSummary); ok {
						level.Debug(c.logger).Log("summary", summary)
						stats.RecordWithTags(ctx,
							[]tag.Mutator{tag.Upsert(stackdriver.StatusTag, "0")},
							stackdriver.PointCount.M(int64(summary.SuccessPointCount)))
						for _, e := range summary.Errors {
							stats.RecordWithTags(ctx,
								[]tag.Mutator{tag.Upsert(stackdriver.StatusTag, fmt.Sprint(uint32(e.Status.Code)))},
								stackdriver.PointCount.M(int64(e.PointCount)))
						}
					}
				}
				switch status.Code() {
				// codes.DeadlineExceeded:
				//   It is safe to retry
				//   google.monitoring.v3.MetricService.CreateTimeSeries
				//   requests with backoff because QueueManager
				//   enforces in-order writes on a time series, which
				//   is a requirement for Stackdriver monitoring.
				//
				// codes.Unavailable:
				//   The condition is most likely transient. The request can
				//   be retried with backoff.
				case codes.DeadlineExceeded, codes.Unavailable:
					errors <- recoverableError{err}
				default:
					errors <- err
				}
			}

			/*
				_, err := service.CreateTimeSeries(ctx, req_copy)
				if err == nil {
					// The response is empty if all points were successfully written.
					stats.RecordWithTags(ctx,
						[]tag.Mutator{tag.Upsert(stackdriver.StatusTag, "0")},
						stackdriver.PointCount.M(int64(end-begin)))
				} else {
					level.Debug(c.logger).Log(
						"msg", "Partial failure calling CreateTimeSeries",
						"err", err)
					status, ok := status.FromError(err)
					if !ok {
						level.Warn(c.logger).Log("msg", "Unexpected error message type from Monitoring API", "err", err)
						errors <- err
						return
					}
					for _, details := range status.Details() {
						if summary, ok := details.(*monitoring.CreateTimeSeriesSummary); ok {
							level.Debug(c.logger).Log("summary", summary)
							stats.RecordWithTags(ctx,
								[]tag.Mutator{tag.Upsert(stackdriver.StatusTag, "0")},
								stackdriver.PointCount.M(int64(summary.SuccessPointCount)))
							for _, e := range summary.Errors {
								stats.RecordWithTags(ctx,
									[]tag.Mutator{tag.Upsert(stackdriver.StatusTag, fmt.Sprint(uint32(e.Status.Code)))},
									stackdriver.PointCount.M(int64(e.PointCount)))
							}
						}
					}
					switch status.Code() {
					// codes.DeadlineExceeded:
					//   It is safe to retry
					//   google.monitoring.v3.MetricService.CreateTimeSeries
					//   requests with backoff because QueueManager
					//   enforces in-order writes on a time series, which
					//   is a requirement for Stackdriver monitoring.
					//
					// codes.Unavailable:
					//   The condition is most likely transient. The request can
					//   be retried with backoff.
					case codes.DeadlineExceeded, codes.Unavailable:
						errors <- nil // recoverableError{err}
					default:
						errors <- err
					}
				}
			*/
		}(i, end)
	}
	wg.Wait()
	close(errors)
	if err, ok := <-errors; ok {
		return err
	}
	return nil
}

func (c Client) Close() error {
	/*
		if c.conn == nil {
			return nil
		}
		return c.conn.Close()
	*/
	return nil
}

func m() {
	body := datadogV2.MetricPayload{
		Series: []datadogV2.MetricSeries{
			{
				Metric: "system.load.1",
				Type:   datadogV2.METRICINTAKETYPE_UNSPECIFIED.Ptr(),
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: datadog.PtrInt64(time.Now().Unix()),
						Value:     datadog.PtrFloat64(0.7),
					},
				},
				Resources: []datadogV2.MetricResource{
					{
						Name: datadog.PtrString("dummyhost"),
						Type: datadog.PtrString("host"),
					},
				},
			},
		},
	}
	ctx := datadog.NewDefaultContext(context.Background())
	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)
	api := datadogV2.NewMetricsApi(apiClient)
	resp, r, err := api.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `MetricsApi.SubmitMetrics`: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}

	responseContent, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(os.Stdout, "Response from `MetricsApi.SubmitMetrics`:\n%s\n", responseContent)
}
