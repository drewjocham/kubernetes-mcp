package security

import (
	"testing"

	"kube-watcher/watcher/internal/events"

	"github.com/stretchr/testify/assert"
)

func TestSanitizer_SanitizeEvent_Disabled(t *testing.T) {
	s := NewSanitizer(false)
	evt := &events.ResourceEvent{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test",
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"env": []interface{}{
							map[string]interface{}{
								"name":  "API_KEY",
								"value": "secret123",
							},
						},
					},
				},
			},
		},
	}
	result := s.SanitizeEvent(evt)
	assert.Equal(t, evt, result)
}

func TestSanitizer_SanitizeEvent_RedactsSensitiveFields(t *testing.T) {
	s := NewSanitizer(true)
	evt := &events.ResourceEvent{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test",
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"env": []interface{}{
							map[string]interface{}{
								"name":  "API_KEY",
								"value": "secret123",
							},
							map[string]interface{}{
								"name":  "LOG_LEVEL",
								"value": "debug",
							},
						},
					},
				},
				"imagePullSecrets": []interface{}{
					map[string]interface{}{
						"name": "regcred",
					},
				},
			},
			"status": map[string]interface{}{},
		},
	}
	result := s.SanitizeEvent(evt)
	assert.NotNil(t, result)
	assert.Nil(t, result.Raw)
	t.Logf("Result object: %+v", result.Object)

	// Check that sensitive env var value is redacted
	spec, ok := result.Object["spec"].(map[string]interface{})
	assert.True(t, ok, "spec should be a map")
	t.Logf("spec: %+v", spec)
	containers, ok := spec["containers"].([]interface{})
	assert.True(t, ok, "containers should be a slice")
	t.Logf("containers: %+v", containers)
	container, ok := containers[0].(map[string]interface{})
	assert.True(t, ok, "container should be a map")
	env, ok := container["env"].([]interface{})
	assert.True(t, ok, "env should be a slice")

	// API_KEY env var should have redacted value
	env0, ok := env[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "API_KEY", env0["name"])
	assert.Equal(t, "[REDACTED]", env0["value"])

	// LOG_LEVEL env var should keep its value
	env1, ok := env[1].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "LOG_LEVEL", env1["name"])
	assert.Equal(t, "debug", env1["value"])

	// imagePullSecrets field should be redacted entirely (deny list)
	ips, ok := spec["imagePullSecrets"].(string)
	assert.True(t, ok, "imagePullSecrets should be redacted string")
	assert.Equal(t, "[REDACTED]", ips)
}

func TestSanitizer_SanitizeEvent_RedactsSensitiveValuePatterns(t *testing.T) {
	s := NewSanitizer(true)
	evt := &events.ResourceEvent{
		Object: map[string]interface{}{
			"data": map[string]interface{}{
				"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
		},
	}
	result := s.SanitizeEvent(evt)
	data, ok := result.Object["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "[REDACTED]", data["token"])
}

func TestSanitizer_SanitizeEvent_RedactsSensitiveFieldNames(t *testing.T) {
	s := NewSanitizer(true)
	evt := &events.ResourceEvent{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"secretRef": map[string]interface{}{
					"name": "my-secret",
				},
			},
		},
	}
	result := s.SanitizeEvent(evt)
	spec, ok := result.Object["spec"].(map[string]interface{})
	assert.True(t, ok)
	secretRef, ok := spec["secretRef"].(string)
	assert.True(t, ok)
	assert.Equal(t, "[REDACTED]", secretRef)
}

func TestSanitizer_SanitizeEvent_HandlesNilObject(t *testing.T) {
	s := NewSanitizer(true)
	evt := &events.ResourceEvent{
		Kind: "Pod",
		Name: "test",
		// Object is nil
	}
	result := s.SanitizeEvent(evt)
	assert.Equal(t, evt, result)
}

func TestSanitizer_SanitizeEvent_ClearsRawField(t *testing.T) {
	s := NewSanitizer(true)
	evt := &events.ResourceEvent{
		Kind: "Pod",
		Name: "test",
		Object: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
		Raw: "some-runtime-object",
	}
	result := s.SanitizeEvent(evt)
	assert.Nil(t, result.Raw)
}
