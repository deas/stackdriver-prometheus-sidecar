/*
 * Unless explicitly stated otherwise all files in this repository are licensed under the Apache-2.0 License.
 * This product includes software developed at Datadog (https://www.datadoghq.com/).
 * Copyright 2019-Present Datadog, Inc.
 */

package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/Stackdriver/stackdriver-prometheus-sidecar/test"
	"gopkg.in/h2non/gock.v1"
)

func setupGock(ctx context.Context, t *testing.T, fixtureFile string, method string, uriPath string) []byte {
	fixturePath, _ := filepath.Abs(fmt.Sprintf("fixtures/%s", fixtureFile))
	dat, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Errorf("Failed to open fixture file: %s", err)
	}
	URL, err := test.Client(ctx).GetConfig().ServerURLWithContext(ctx, "")
	if err != nil {
		t.Errorf("Failed to generate URL: %s", err)
	}
	x := gock.New(URL)
	switch strings.ToLower(method) {
	case "get":
		x.Get(uriPath)
	case "post":
		x.Post(uriPath)
	case "put":
		x.Put(uriPath)
	case "delete":
		x.Put(uriPath)
	}

	x.Reply(200).JSON(dat)
	return dat
}

/*
func TestMetrics(t *testing.T) {
	// ctx, err := test.WithClock(test.WithClient(context.Background()), test.SecurePath(t.Name()))
	ctx := test.WithClient(context.Background())
	// ctx, finish := tests.WithTestSpan(context.Background(), t)
	// defer finish()
	ctx = test.WithTestAuth(ctx)
	// ctx, finish = test.WithRecorder(test.WithTestAuth(ctx), t)
	// defer finish()
	assert := test.Assert(ctx, t)
	api := datadogV1.NewMetricsApi(test.Client(ctx))
	now := clockwork.NewRealClock().Now().Unix()
	// now := test.ClockFromContext(ctx).Now().Unix()

	// the API would replace everything by underscores anyway, making us unable to search for metric by this value
	// in the tests.Retry loop below
	testMetric := "foo" // strings.ReplaceAll(*test.UniqueEntityName(ctx, t), "-", "_")
	testQuery := fmt.Sprintf("avg:%s{bar:baz}by{host}", testMetric)

	metricsPayload := fmt.Sprintf(
		`{"series": [{"host": "go-client-test-host", "metric": "%s", "points": [[%f, 10.5], [%f, 11]], "tags": ["%s", "%s"]}]}`,
		testMetric, float64(now-60), float64(now), "tag:foo", "bar:baz",
	)
	httpresp, respBody, err := test.SendRequest(ctx, "POST", "/api/v1/series", []byte(metricsPayload))
	if err != nil {
		t.Fatalf("Error submitting metric: Response %s: %v", string(respBody), err)
	}
	// assert.Equal(httpresp.StatusCode, 202)
	assert.Equal(202, httpresp.StatusCode)
	assert.Equal(`{"status": "ok"}`, string(respBody))

	// Check that the metric was submitted successfully
	err = test.Retry(10*time.Second, 10, func() bool {
		metrics, httpresp, err := api.ListActiveMetrics(ctx, now)
		if err != nil {
			t.Logf("Error getting list of active metrics: Response %s: %v", err.(datadog.GenericOpenAPIError).Body(), err)
			return false
		}
		if httpresp.StatusCode != 200 {
			return false
		}

		found := false
		for _, metric := range metrics.GetMetrics() {
			if metric == testMetric {
				found = true
				break
			}
		}
		return found
	})

	if err != nil {
		t.Fatalf("%v", err)
	}

	// Test query
	queryResult, httpresp, err := api.QueryMetrics(ctx, now-100, now+100, testQuery)
	if err != nil {
		t.Errorf("Error making query %s: Response %s: %v", testQuery, err.(datadog.GenericOpenAPIError).Body(), err)
	}
	assert.Equal(200, httpresp.StatusCode)
	assert.Equal([]string{"host"}, queryResult.GetGroupBy())
	assert.Equal(testQuery, queryResult.GetQuery())
	assert.Equal((now-100)*1000, queryResult.GetFromDate())
	assert.Equal((now+100)*1000, queryResult.GetToDate())
	assert.Equal("ok", queryResult.GetStatus())
	assert.Equal("time_series", queryResult.GetResType())
	assert.Equal(1, len(queryResult.GetSeries()))
	series := queryResult.GetSeries()[0]
	assert.Equal(int64(2), series.GetLength())
	assert.Equal("avg", series.GetAggr())
	assert.Equal(testMetric, series.GetDisplayName())
	assert.Equal(testMetric, series.GetMetric())
	assert.Equal(series.GetPointlist()[0][0], datadog.PtrFloat64(float64(series.GetStart())))
	assert.Equal(series.GetPointlist()[1][0], datadog.PtrFloat64(float64(series.GetEnd())))
	assert.Equal(10.5, *series.GetPointlist()[0][1])
	assert.Equal(11., *series.GetPointlist()[1][1])

	// Test search
	searchQuery := fmt.Sprintf("metrics:%s", testMetric)
	searchResult, httpresp, err := api.ListMetrics(ctx, searchQuery)
	if err != nil {
		t.Errorf("Error searching metrics %s: Response %s: %v", searchQuery, err.(datadog.GenericOpenAPIError).Body(), err)
	}
	assert.Equal(200, httpresp.StatusCode)
	metrics := searchResult.Results.GetMetrics()
	assert.Equal(1, len(metrics))
	assert.Equal(testMetric, metrics[0])

	// Test metric metadata
	metadata, httpresp, err := api.GetMetricMetadata(ctx, testMetric)
	if err != nil {
		t.Errorf("Error getting metric metadata for %s: Response %s: %v", testMetric, err.(datadog.GenericOpenAPIError).Body(), err)
	}
	assert.Equal(200, httpresp.StatusCode)
	assert.Nil(metadata.Description)
	assert.Nil(metadata.Integration)
	assert.Nil(metadata.PerUnit)
	assert.Nil(metadata.Unit)
	assert.Nil(metadata.ShortName)
	assert.Nil(metadata.StatsdInterval)
	assert.Nil(metadata.Type)

	newMetadata := datadogV1.MetricMetadata{
		Description:    datadog.PtrString("description"),
		PerUnit:        datadog.PtrString("second"),
		Unit:           datadog.PtrString("byte"),
		ShortName:      datadog.PtrString("short_name"),
		StatsdInterval: datadog.PtrInt64(20),
		Type:           datadog.PtrString("count"),
	}

	metadata, httpresp, err = api.UpdateMetricMetadata(ctx, testMetric, newMetadata)
	if err != nil {
		t.Errorf("Error editing metric metadata for %s: Response %s: %v", testMetric, err.(datadog.GenericOpenAPIError).Body(), err)
	}
	assert.Equal(200, httpresp.StatusCode)
	assert.Equal("description", metadata.GetDescription())
	assert.Nil(metadata.Integration)
	assert.Equal("second", metadata.GetPerUnit())
	assert.Equal("byte", metadata.GetUnit())
	assert.Equal("short_name", metadata.GetShortName())
	assert.Equal(int64(20), metadata.GetStatsdInterval())
	assert.Equal("count", metadata.GetType())
}
*/

func TestMetricListActive(t *testing.T) {
	// ctx, finish := tests.WithTestSpan(context.Background(), t)
	// defer finish()
	ctx := test.WithClient(test.WithFakeAuth(context.Background()))
	assert := test.Assert(ctx, t)
	// data :=
	setupGock(ctx, t, "metrics/202.json", "POST", "api/v2/series")
	// data := setupGock(ctx, t, "metrics/active_metrics.json", "GET", "metrics")
	defer gock.Off()

	// datadogV2.MetricResource
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

	// var expected datadogV1.MetricsListResponse
	// json.Unmarshal([]byte(data), &expected)

	api := datadogV2.NewMetricsApi(test.Client(ctx))
	resp, _, err := api.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())

	assert.Nil(err)
	assert.Empty(resp.Errors)
	/*
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error when calling `MetricsApi.SubmitMetrics`: %v\n", err)
			fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
		}
	*/

	responseContent, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(os.Stdout, "Response from `MetricsApi.SubmitMetrics`:\n%s\n", responseContent)
}

/*
func TestMetricsListActive400Error(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()
	// Setup the Client we'll use to interact with the Test account
	ctx = tests.WithClient(tests.WithFakeAuth(ctx))
	assert := tests.Assert(ctx, t)
	api := datadogV1.NewMetricsApi(tests.Client(ctx))

	// Error 400 cannot be triggered from the client due to client side validation, so mock it
	res, err := tests.ReadFixture("fixtures/metrics/error_400.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %s", err)
	}
	URL, err := tests.Client(ctx).GetConfig().ServerURLWithContext(ctx, "")
	assert.NoError(err)
	gock.New(URL).Get("/api/v1/metrics").Reply(400).JSON(res)
	defer gock.Off()

	_, httpresp, err := api.ListActiveMetrics(ctx, -1)
	assert.Equal(400, httpresp.StatusCode)
	apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
	assert.True(ok)
	assert.NotEmpty(apiError.GetErrors())
}

func TestMetricsListActiveErrors(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()

	testCases := map[string]struct {
		Ctx                func(context.Context) context.Context
		ExpectedStatusCode int
	}{
		// Error 400 cannot be triggered from the client due to client side validation, so mock it
		// "400 Bad Request": {WithTestAuth,400},
		"403 Forbidden": {tests.WithFakeAuth, 403},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, finish := tests.WithRecorder(tc.Ctx(ctx), t)
			defer finish()
			assert := tests.Assert(ctx, t)
			api := datadogV1.NewMetricsApi(tests.Client(ctx))

			_, httpresp, err := api.ListActiveMetrics(ctx, -1)
			assert.Equal(tc.ExpectedStatusCode, httpresp.StatusCode)
			apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
			assert.True(ok)
			assert.NotEmpty(apiError.GetErrors())
		})
	}
}

func TestMetricsMetadataGetErrors(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()

	testCases := map[string]struct {
		Ctx                func(context.Context) context.Context
		ExpectedStatusCode int
	}{
		"403 Forbidden": {tests.WithFakeAuth, 403},
		"404 Not Found": {tests.WithTestAuth, 404},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, finish := tests.WithRecorder(tc.Ctx(ctx), t)
			defer finish()
			assert := tests.Assert(ctx, t)
			api := datadogV1.NewMetricsApi(tests.Client(ctx))

			_, httpresp, err := api.GetMetricMetadata(ctx, "ametric")
			assert.Equal(tc.ExpectedStatusCode, httpresp.StatusCode)
			apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
			assert.True(ok)
			assert.NotEmpty(apiError.GetErrors())
		})
	}
}

func TestMetricsMetadataUpdate400Error(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()
	// Setup the Client we'll use to interact with the Test account
	ctx = tests.WithClient(tests.WithFakeAuth(ctx))
	assert := tests.Assert(ctx, t)
	api := datadogV1.NewMetricsApi(tests.Client(ctx))

	// Error 400 cannot be triggered from the client due to client side validation, so mock it
	res, err := tests.ReadFixture("fixtures/metrics/error_400.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %s", err)
	}
	URL, err := tests.Client(ctx).GetConfig().ServerURLWithContext(ctx, "")
	assert.NoError(err)
	gock.New(URL).Put("/api/v1/metrics/ametric").Reply(400).JSON(res)
	defer gock.Off()

	_, httpresp, err := api.UpdateMetricMetadata(ctx, "ametric", datadogV1.MetricMetadata{})
	assert.Equal(400, httpresp.StatusCode)
	apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
	assert.True(ok)
	assert.NotEmpty(apiError.GetErrors())
}

func TestMetricsMetadataUpdateErrors(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()

	testCases := map[string]struct {
		Ctx                func(context.Context) context.Context
		Body               datadogV1.MetricMetadata
		ExpectedStatusCode int
	}{
		"403 Forbidden": {tests.WithFakeAuth, datadogV1.MetricMetadata{}, 403},
		"404 Not Found": {tests.WithTestAuth, datadogV1.MetricMetadata{}, 404},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, finish := tests.WithRecorder(tc.Ctx(ctx), t)
			defer finish()
			assert := tests.Assert(ctx, t)
			api := datadogV1.NewMetricsApi(tests.Client(ctx))

			_, httpresp, err := api.UpdateMetricMetadata(ctx, "ametric", tc.Body)
			assert.Equal(tc.ExpectedStatusCode, httpresp.StatusCode)
			apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
			assert.True(ok)
			assert.NotEmpty(apiError.GetErrors())
		})
	}
}

func TestMetricsList400Error(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()
	// Setup the Client we'll use to interact with the Test account
	ctx = tests.WithClient(tests.WithFakeAuth(ctx))
	assert := tests.Assert(ctx, t)
	api := datadogV1.NewMetricsApi(tests.Client(ctx))

	// Error 400 cannot be triggered from the client due to client side validation, so mock it
	res, err := tests.ReadFixture("fixtures/metrics/error_400.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %s", err)
	}
	URL, err := Client(ctx).GetConfig().ServerURLWithContext(ctx, "")
	assert.NoError(err)
	gock.New(URL).Get("/api/v1/search").Reply(400).JSON(res)
	defer gock.Off()

	_, httpresp, err := api.ListMetrics(ctx, "")
	assert.Equal(400, httpresp.StatusCode)
	apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
	assert.True(ok)
	assert.NotEmpty(apiError.GetErrors())
}

func TestMetricsListErrors(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()

	testCases := map[string]struct {
		Ctx                func(context.Context) context.Context
		ExpectedStatusCode int
	}{
		// Error 400 cannot be triggered from the client due to client side validation
		// "400 Bad Request": {WithTestAuth,400},
		"403 Forbidden": {tests.WithFakeAuth, 403},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, finish := tests.WithRecorder(tc.Ctx(ctx), t)
			defer finish()
			assert := tests.Assert(ctx, t)
			api := datadogV1.NewMetricsApi(tests.Client(ctx))

			_, httpresp, err := api.ListMetrics(ctx, "somequery")
			assert.Equal(tc.ExpectedStatusCode, httpresp.StatusCode)
			apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
			assert.True(ok)
			assert.NotEmpty(apiError.GetErrors())
		})
	}
}

func TestMetricsQuery400Error(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()
	// Setup the Client we'll use to interact with the Test account
	ctx = tests.WithClient(tests.WithFakeAuth(ctx))
	assert := tests.Assert(ctx, t)
	api := datadogV1.NewMetricsApi(tests.Client(ctx))

	// Error 400 cannot be triggered from the client due to client side validation, so mock it
	res, err := tests.ReadFixture("fixtures/metrics/error_400.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %s", err)
	}
	URL, err := tests.Client(ctx).GetConfig().ServerURLWithContext(ctx, "")
	assert.NoError(err)
	gock.New(URL).Get("/api/v1/query").Reply(400).JSON(res)
	defer gock.Off()

	_, httpresp, err := api.QueryMetrics(ctx, 0, 0, "")
	assert.Equal(400, httpresp.StatusCode)
	apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
	assert.True(ok)
	assert.NotEmpty(apiError.GetErrors())
}

func TestMetricsQueryErrors(t *testing.T) {
	ctx, finish := tests.WithTestSpan(context.Background(), t)
	defer finish()

	testCases := map[string]struct {
		Ctx                func(context.Context) context.Context
		ExpectedStatusCode int
	}{
		// Error 400 cannot be triggered from the client due to client side validation
		// "400 Bad Request": {WithTestAuth,400},
		"403 Forbidden": {tests.WithFakeAuth, 403},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, finish := tests.WithRecorder(tc.Ctx(ctx), t)
			defer finish()
			assert := tests.Assert(ctx, t)
			api := datadogV1.NewMetricsApi(tests.Client(ctx))

			_, httpresp, err := api.QueryMetrics(ctx, 0, 0, "somequery")
			assert.Equal(tc.ExpectedStatusCode, httpresp.StatusCode)
			apiError, ok := err.(datadog.GenericOpenAPIError).Model().(datadogV1.APIErrorResponse)
			assert.True(ok)
			assert.NotEmpty(apiError.GetErrors())
		})
	}
}
*/
