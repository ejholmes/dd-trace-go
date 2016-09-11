package tracer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONEncoder(t *testing.T) {
	assert := assert.New(t)

	// create a spans list with a single span
	var spans []*Span
	span := newSpan(0, 0, 0, "pylons", "pylons.request", "/", nil)
	span.Start = 0
	spans = append(spans, span)

	// the encoder must return a valid JSON byte array
	const want = `[{"name":"pylons.request","service":"pylons","resource":"/","type":"","start":0,"duration":0,"error":0,"meta":null,"metrics":null,"span_id":0,"trace_id":0,"parent_id":0}]`
	encoder := NewJSONEncoder()
	res, err := encoder.Encode(spans)
	assert.Nil(err)
	assert.Equal(string(res), want)
}
