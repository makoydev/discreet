package audit

import "encoding/json"

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

func jsonMarshal(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
