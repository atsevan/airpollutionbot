package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// OWMApiEndpoint is an base apiEndpoint
const OWMApiEndpoint = "http://api.openweathermap.org/data/2.5/"

var (
	aqiDesc = map[AirQualityIndex]string{
		1: "🟩 (Good)",
		2: "🟨 (Fair)",
		3: "🟧 (Moderate)",
		4: "🟥 (Poor)",
		5: "⬛ (Very Poor)",
	}
	aqiDescription = map[AirQualityIndex]string{
		1: "No health implications.",
		2: "Some pollutants may slightly affect very few hypersensitive individuals.",
		3: "Healthy people may experience slight irritations and sensitive individuals will be slightly affected to a larger extent.",
		4: "Sensitive individuals will experience more serious conditions. The hearts and respiratory systems of healthy people may be affected.",
		5: "Healthy people will commonly show symptoms. People with respiratory or heart diseases will be significantly affected and will experience reduced endurance in activities.",
	}
)

// HTTPClient is the type needed for the bot to perform HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// OpenWheatherMapApi keeps the infromation for openweathermap.org API communication
type OpenWheatherMapApi struct {
	token       string
	httpClient  HTTPClient
	Debug       bool
	apiEndpoint string
}

// NewOpenWheatherMapApi creates a new clinet for OpenWheatherMapApi
func NewOpenWheatherMapApi(token string) (*OpenWheatherMapApi, error) {
	return &OpenWheatherMapApi{token, &http.Client{}, false, OWMApiEndpoint}, nil
}

func (owma *OpenWheatherMapApi) makeRequest(path string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s&appid=%s", owma.apiEndpoint, path, owma.token)
	if owma.Debug {
		log.Printf("air_pollution url: %q", url)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}
	resp, err := owma.httpClient.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}
	return body, nil
}

// GetAirPollution gets the current information about air pollution for the coordintes.
// returns ApiPollutionResponse or Error
func (owma *OpenWheatherMapApi) GetAirPollution(l *Location) (*ApiPollutionResponse, error) {
	path := fmt.Sprintf("air_pollution?lat=%f&lon=%f", l.Latitude, l.Longitude)
	data, err := owma.makeRequest(path)
	if err != nil {
		return &ApiPollutionResponse{}, err
	}
	var apiResp ApiPollutionResponse
	err = json.Unmarshal(data, &apiResp)
	if err != nil {
		return &ApiPollutionResponse{}, err
	}
	if owma.Debug {
		log.Printf("air_pollution response: %v", &apiResp)
	}
	return &apiResp, nil
}

// Location keeps coordinates for the result
type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// AirQualityIndex is the current level of Air Quality
type AirQualityIndex int

func (aqi AirQualityIndex) String() string {
	return aqiDesc[aqi]
}

// Description returns a longer description of the Air Quality Index level
func (aqi AirQualityIndex) Description() string {
	return aqiDescription[aqi]
}

// DataPoint keeps the AirPollutionIndex measurement
type DataPoint struct {
	Dt   int64 `json:"dt"`
	Main struct {
		Aqi AirQualityIndex `json:"aqi"`
	} `json:"main"`
	Components map[string]float64 `json:"components"` // Components keeps concentration of each component in μg/m3
}

// GetAQI returns the AirQualityIndex for the DataPoint
func (dp *DataPoint) GetAQI() AirQualityIndex {
	return dp.Main.Aqi
}

// ApiPollutionResponse contains the infromation about AirQualityIndex and components for a location
// see https://openweathermap.org/api/air-pollution#fields
type ApiPollutionResponse struct {
	Location Location    `json:"coord"`
	DP       []DataPoint `json:"list"`
}
