/**
 * InfuxDB v2 client
 *
 * Copyright © 2024-2026 MaximAL
 *
 */

package influx

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type InfluxConfig struct {
	Enabled   bool
	Version   uint8
	Host      string
	Token     string
	Org       string
	Bucket    string
	Database  string
	Precision string
	Comments  bool
	Tags      map[string]string
}

type FieldType uint8

const (
	TypeFloat  FieldType = 1
	TypeInt    FieldType = 2
	TypeUint   FieldType = 3
	TypeBool   FieldType = 4
	TypeString FieldType = 5
)

type Field struct {
	Name        string
	Type        FieldType
	Value       any
	FloatValue  float64
	IntValue    int64
	UintValue   uint64
	BoolValue   bool
	StringValue string
}

type Metric struct {
	Name      string
	Fields    []Field
	Tags      map[string]string
	Comment   string
	Timestamp uint64
}

var commonTags map[string]string = map[string]string{}
var metrics []Metric = []Metric{}

func SetCommonTags(tags map[string]string) {
	commonTags = tags
}

func ResetMetrics() {
	metrics = metrics[:0]
}

func AddMetric(name string, fields []Field, tags map[string]string, comment string) {
	if name == "" {
		// Error
	}
	if len(fields) == 0 {
		// Error
	}
	metrics = append(metrics, Metric{
		Name:    name,
		Fields:  fields,
		Tags:    tags,
		Comment: comment,
	})
}

func AddValueMetric(name string, value any, tags map[string]string, comment string) {
	// Type assertion
	switch value := value.(type) {
	default:
		// fmt.Printf("unexpected type %T\n", t) // %T prints whatever type t has
		panic("Unexpected type; must be one of: float64, int64, uint64, bool, string")
	case float64:
		AddFloatMetric(name, value, tags, comment)
	case int64:
		AddIntMetric(name, value, tags, comment)
	case uint64:
		AddUintMetric(name, value, tags, comment)
	case bool:
		AddBoolMetric(name, value, tags, comment)
	case string:
		AddStringMetric(name, value, tags, comment)
	}
}

func AddFloatMetric(name string, value float64, tags map[string]string, comment string) {
	metrics = append(metrics, Metric{
		Name:    name,
		Fields:  []Field{{Name: "value", Type: TypeFloat, FloatValue: value}},
		Tags:    tags,
		Comment: comment,
	})
}

func AddIntMetric(name string, value int64, tags map[string]string, comment string) {
	metrics = append(metrics, Metric{
		Name:    name,
		Fields:  []Field{{Name: "value", Type: TypeInt, IntValue: value}},
		Tags:    tags,
		Comment: comment,
	})
}

func AddUintMetric(name string, value uint64, tags map[string]string, comment string) {
	metrics = append(metrics, Metric{
		Name:    name,
		Fields:  []Field{{Name: "value", Type: TypeUint, UintValue: value}},
		Tags:    tags,
		Comment: comment,
	})
}

func AddBoolMetric(name string, value bool, tags map[string]string, comment string) {
	metrics = append(metrics, Metric{
		Name:    name,
		Fields:  []Field{{Name: "value", Type: TypeBool, BoolValue: value}},
		Tags:    tags,
		Comment: comment,
	})
}

func AddStringMetric(name string, value string, tags map[string]string, comment string) {
	metrics = append(metrics, Metric{
		Name:    name,
		Fields:  []Field{{Name: "value", Type: TypeString, StringValue: value}},
		Tags:    tags,
		Comment: comment,
	})
}

func GetMetricsText(withComments bool) string {
	count := GetMetricsCount()
	if count == 0 {
		return ""
	}

	lines := make([]string, 0, count)
	if withComments {
		lines = append(lines, "## metrics start: "+strconv.FormatUint(count, 10))
	}

	for _, metric := range metrics {
		tags := []string{escapeMetricName(metric.Name)}
		for key, value := range commonTags {
			tags = append(tags, escapeKey(key)+"="+escapeTagValue(value))
		}
		for key, value := range metric.Tags {
			tags = append(tags, escapeKey(key)+"="+escapeTagValue(value))
		}

		// Format values to match field types
		values := []string{}
		for _, field := range metric.Fields {
			var escaped string
			switch field.Type {
			case TypeFloat:
				escaped = strconv.FormatFloat(field.FloatValue, 'f', -1, 64)
			case TypeInt:
				escaped = strconv.FormatInt(field.IntValue, 10) + "i"
			case TypeUint:
				escaped = strconv.FormatUint(field.UintValue, 10) + "u"
				// QuestDB crutch (compatibility mode)
				//escaped = strconv.FormatUint(field.UintValue, 10) + "i"
			case TypeBool:
				escaped = strconv.FormatBool(field.BoolValue)
			case TypeString:
				escaped = "\"" + escapeStringFieldValue(field.StringValue) + "\""
			}
			values = append(values, escapeKey(field.Name)+"="+escaped)
		}

		parts := []string{
			strings.Join(tags, ","),
			strings.Join(values, ","),
		}

		// Measurement timestamp
		if metric.Timestamp > 0 {
			parts = append(parts, strconv.FormatUint(metric.Timestamp, 10))
		}

		// Measurement comment
		if withComments && metric.Comment != "" {
			lines = append(lines, "# "+strings.ReplaceAll(metric.Comment, "\n", " "))
		}

		// Measurement line: name, tags, fields/values
		lines = append(lines, strings.Join(parts, "    "))
		// QuestDB crutch (compatibility mode)
		//lines = append(lines, strings.Join(parts, " "))
	}

	if withComments {
		lines = append(lines, "## metrics end")
	}

	return strings.Join(lines, "\n")
}

func GetMetricsCount() uint64 {
	return uint64(len(metrics))
}

// Escape a metric/measurement name
//
// https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/#special-characters
func escapeMetricName(name string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(name, ",", "\\,"),
		" ",
		"\\ ",
	)
}

// Escape a tag’s or field’s key
//
// https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/#special-characters
func escapeKey(key string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(key, ",", "\\,"),
			"=",
			"\\=",
		),
		" ",
		"\\ ",
	)
}

// Escape a tag’s value
//
// https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/#special-characters
func escapeTagValue(value string) string {
	return escapeKey(value)
}

// Escape a string field’s value
//
// https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/#special-characters
func escapeStringFieldValue(value string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(value, "\"", "\\\""),
		"\\",
		"\\\\",
	)
}

func SendMetrics(config InfluxConfig) bool {
	return SendText(GetMetricsText(config.Comments), config)
}

func SendText(text string, config InfluxConfig) bool {
	if config.Version == 2 {
		return SendText2(text, config)
	}
	return SendText3(text, config)
}

func SendText2(text string, config InfluxConfig) bool {
	var host string
	if strings.HasPrefix(config.Host, "https://") || strings.HasPrefix(config.Host, "http://") {
		host = config.Host
	} else {
		host = "http://" + config.Host
	}

	params := url.Values{}
	params.Add("org", config.Org)
	params.Add("bucket", config.Bucket)
	params.Add("precision", config.Precision)

	url := host + "/api/v2/write?" + params.Encode()
	request, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(text)))
	if err != nil {
		println(err)
		return false
	}
	request.Header.Set("Authorization", "Token "+config.Token)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		//panic(err)
		println(err)
		return false
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		println("HTTP", response.Status)
		println("Request: POST", url)
		//println("Response headers:", response.Header)
		body, _ := io.ReadAll(response.Body)
		println("Response Body:", string(body))
		return false
	}

	return true
}

func SendText3(text string, config InfluxConfig) bool {
	var host string
	if strings.HasPrefix(config.Host, "https://") || strings.HasPrefix(config.Host, "http://") {
		host = config.Host
	} else {
		host = "http://" + config.Host
	}

	params := url.Values{}
	if config.Database != "" {
		params.Add("db", config.Database)
	} else {
		params.Add("db", config.Bucket)
	}

	switch config.Precision {
	case "s":
		params.Add("precision", "second")
	case "ms":
		params.Add("precision", "millisecond")
	case "us":
		params.Add("precision", "microsecond")
	case "ns":
		params.Add("precision", "nanosecond")
	default:
		params.Add("precision", "auto")
	}

	url := host + "/api/v3/write_lp?" + params.Encode()
	request, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(text)))
	if err != nil {
		println(err)
		return false
	}
	request.Header.Set("Authorization", "Bearer "+config.Token)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		//panic(err)
		println(err)
		return false
	}
	defer response.Body.Close()

	if response.StatusCode != 204 {
		println("HTTP", response.Status)
		println("Request: POST", url)
		//println("Response headers:", response.Header)
		body, _ := io.ReadAll(response.Body)
		println("Response Body:", string(body))
		return false
	}

	return true
}
