package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-graphite/carbonapi/expr"
	pb "github.com/go-graphite/carbonzipper/carbonzipperpb3"
	realZipper "github.com/go-graphite/carbonzipper/zipper"
	"github.com/lomik/zapwriter"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockCarbonZipper struct {
	z *realZipper.Zipper

	logger      *zap.Logger
	statsSender func(*realZipper.Stats)
}

func newMockCarbonZipper() *mockCarbonZipper {
	z := &mockCarbonZipper{}

	return z
}

func (z mockCarbonZipper) Find(ctx context.Context, metric string) (pb.GlobResponse, error) {
	return getGlobResponse(), nil
}

func (z mockCarbonZipper) Info(ctx context.Context, metric string) (map[string]pb.InfoResponse, error) {
	response := getMockInfoResponse()

	return response, nil
}

func (z mockCarbonZipper) Render(ctx context.Context, metric string, from, until int32) ([]*expr.MetricData, error) {
	var result []*expr.MetricData
	multiFetchResponse := getMultiFetchResponse()
	result = append(result, &expr.MetricData{FetchResponse: multiFetchResponse.Metrics[0]})
	return result, nil
}

func getGlobResponse() pb.GlobResponse {
	globMtach := pb.GlobMatch{Path: "foo.bar", IsLeaf: true}
	var matches []pb.GlobMatch
	matches = append(matches, globMtach)
	globResponse := pb.GlobResponse{
		Name:    "foo.bar",
		Matches: matches,
	}
	return globResponse
}

func getMultiFetchResponse() pb.MultiFetchResponse {
	mfr := pb.FetchResponse{
		Name:      "foo.bar",
		StartTime: 1510913280,
		StopTime:  1510913880,
		StepTime:  60,
		Values:    []float64{0, 1510913759, 1510913818},
		IsAbsent:  []bool{true, false, false},
	}

	result := pb.MultiFetchResponse{Metrics: []pb.FetchResponse{mfr}}
	return result
}

func getMockInfoResponse() map[string]pb.InfoResponse {
	decoded := make(map[string]pb.InfoResponse)
	r := pb.Retention{
		SecondsPerPoint: 60,
		NumberOfPoints:  43200,
	}
	d := pb.InfoResponse{
		Name:              "foo.bar",
		AggregationMethod: "Average",
		MaxRetention:      157680000,
		XFilesFactor:      0.5,
		Retentions:        []pb.Retention{r},
	}
	decoded["http://127.0.0.1:8080"] = d
	return decoded
}

func init() {
	logger := zapwriter.Logger("main")

	setUpViper(logger)
	setUpConfigUpstreams(logger)
	setUpConfig(logger, newMockCarbonZipper())
	initHandlers()
}

func setUpRequest(t *testing.T, url string) (*http.Request, *httptest.ResponseRecorder) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	return req, rr
}

func TestRenderHandler(t *testing.T) {
	req, rr := setUpRequest(t, "/render/?target=fallbackSeries(foo.bar,foo.baz)&from=-10minutes&format=json")
	renderHandler(rr, req)

	expected := `[{"target":"foo.bar","datapoints":[[null,1510913280],[1510913759,1510913340],[1510913818,1510913400]]}]`

	// Check the status code is what we expect.
	assert.Equal(t, rr.Code, http.StatusOK, "HttpStatusCode should be 200 OK.")
	assert.Equal(t, expected, rr.Body.String(), "Http response should be same.")
}

func TestFindHandler(t *testing.T) {
	req, rr := setUpRequest(t, "/metrics/find/?query=foo.bar&format=json")
	findHandler(rr, req)

	body := rr.Body.String()
	expected := `[{"allowChildren":0,"expandable":0,"leaf":1,"id":"foo.bar","text":"bar","context":{}}]` + "\n"
	assert.Equal(t, rr.Code, http.StatusOK, "HttpStatusCode should be 200 OK.")
	assert.Equal(t, expected, body, "Http response should be same.")
}

func TestInfoHandler(t *testing.T) {
	req, rr := setUpRequest(t, "/info/?target=foo.bar&format=json")
	infoHandler(rr, req)

	body := rr.Body.String()
	expected := getMockInfoResponse()
	expectedJson, err := json.Marshal(expected)
	assert.Nil(t, err)

	assert.Equal(t, rr.Code, http.StatusOK, "HttpStatusCode should be 200 OK.")
	assert.Equal(t, string(expectedJson), body, "Http response should be same.")
}
