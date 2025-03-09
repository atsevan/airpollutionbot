// owmapi_test.go
package main

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func TestGetAirPollution(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			json := `{"coord":{"lon":10,"lat":20},"list":[{"main":{"aqi":2},"components":{"co":200.27,"no":0.02,"no2":0.31,"o3":68.45,"so2":0.26,"pm2_5":2.74,"pm10":3.55,"nh3":0.51},"dt":1605182400}]}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(json)),
			}, nil
		},
	}

	owma := &OpenWheatherMapAPI{
		token:       "test_token",
		httpClient:  mockClient,
		Debug:       true,
		apiEndpoint: "http://test.api",
	}

	location := &Location{Latitude: 20, Longitude: 10}
	resp, err := owma.GetAirPollution(location)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if resp.Location.Latitude != 20 || resp.Location.Longitude != 10 {
		t.Errorf("Expected location {20, 10}, got {%v, %v}", resp.Location.Latitude, resp.Location.Longitude)
	}

	if len(resp.DP) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(resp.DP))
	}

	if resp.DP[0].GetAQI() != 2 {
		t.Errorf("Expected AQI 2, got %d", resp.DP[0].GetAQI())
	}
}

func TestAirQualityIndexString(t *testing.T) {
	tests := []struct {
		aqi      AirQualityIndex
		expected string
	}{
		{1, "🟩 (Good)"},
		{2, "🟨 (Fair)"},
		{3, "🟧 (Moderate)"},
		{4, "🟥 (Poor)"},
		{5, "⬛ (Very Poor)"},
	}

	for _, test := range tests {
		result := test.aqi.String()
		if result != test.expected {
			t.Errorf("For AQI %d, expected %s, got %s", test.aqi, test.expected, result)
		}
	}
}

func TestAirQualityIndexDescription(t *testing.T) {
	tests := []struct {
		aqi      AirQualityIndex
		expected string
	}{
		{1, "No health implications."},
		{2, "Some pollutants may slightly affect very few hypersensitive individuals."},
		{3, "Healthy people may experience slight irritations and sensitive individuals will be slightly affected to a larger extent."},
		{4, "Sensitive individuals will experience more serious conditions. The hearts and respiratory systems of healthy people may be affected."},
		{5, "Healthy people will commonly show symptoms. People with respiratory or heart diseases will be significantly affected and will experience reduced endurance in activities."},
	}

	for _, test := range tests {
		result := test.aqi.Description()
		if result != test.expected {
			t.Errorf("For AQI %d, expected description %s, got %s", test.aqi, test.expected, result)
		}
	}
}

func TestAirQualityIndexColor(t *testing.T) {
	tests := []struct {
		aqi      AirQualityIndex
		expected string
	}{
		{1, "🟩 (Good)"},
		{2, "🟨 (Fair)"},
		{3, "🟧 (Moderate)"},
		{4, "🟥 (Poor)"},
		{5, "⬛ (Very Poor)"},
	}
	for _, test := range tests {
		result := test.aqi.String()
		if result != test.expected {
			t.Errorf("For AQI %d, expected color %s, got %s", test.aqi, test.expected, result)
		}
	}
}
