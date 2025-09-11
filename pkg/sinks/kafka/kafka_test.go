package kafka

import (
	"context"
	"testing"

	go_metrics "github.com/kentik/go-metrics"
	"github.com/kentik/ktranslate"
	"github.com/kentik/ktranslate/pkg/eggs/logger"
	lt "github.com/kentik/ktranslate/pkg/eggs/logger/testing"
	"github.com/kentik/ktranslate/pkg/formats"
	"github.com/kentik/ktranslate/pkg/kt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKafkaSinkCreation(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Test sink creation
	cfg := &ktranslate.KafkaSinkConfig{
		Topic:            "test-topic",
		BootstrapServers: "localhost:9092",
		SecurityProtocol: "PLAINTEXT",
	}

	registry := go_metrics.NewRegistry()
	log := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	sink, err := NewSink(log, registry, cfg)
	require.NoError(err)
	assert.NotNil(sink)
	assert.Equal("test-topic", cfg.Topic)
	assert.Equal("localhost:9092", cfg.BootstrapServers)
	assert.Equal("PLAINTEXT", cfg.SecurityProtocol)
}

func TestKafkaSinkInitValidation(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	registry := go_metrics.NewRegistry()
	log := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	// Test missing topic
	cfg := &ktranslate.KafkaSinkConfig{
		BootstrapServers: "localhost:9092",
		SecurityProtocol: "PLAINTEXT",
	}

	sink, err := NewSink(log, registry, cfg)
	require.NoError(err)

	ctx := context.Background()
	err = sink.Init(ctx, formats.FORMAT_JSON, kt.CompressionNone, nil)
	assert.Error(err)
	assert.Contains(err.Error(), "no topic set")

	// Test missing bootstrap servers
	cfg = &ktranslate.KafkaSinkConfig{
		Topic:            "test-topic",
		SecurityProtocol: "PLAINTEXT",
	}

	sink, err = NewSink(log, registry, cfg)
	require.NoError(err)

	err = sink.Init(ctx, formats.FORMAT_JSON, kt.CompressionNone, nil)
	assert.Error(err)
	assert.Contains(err.Error(), "no bootstrap servers set")
}

func TestKafkaSinkKerberosConfiguration(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Test Kerberos configuration
	cfg := &ktranslate.KafkaSinkConfig{
		Topic:                   "test-topic",
		BootstrapServers:        "localhost:9094",
		SecurityProtocol:        "SASL_SSL",
		SASLMechanism:          "GSSAPI",
		KerberosServiceName:    "kafka",
		KerberosRealm:          "EXAMPLE.COM",
		KerberosPrincipal:      "ktranslate@EXAMPLE.COM",
		KerberosKeytabPath:     "/etc/keytabs/ktranslate.keytab",
		KerberosConfigPath:     "/etc/krb5.conf",
		KerberosDisablePAFXFAST: false,
		RequiredAcks:           -1,
		Compression:            "snappy",
	}

	registry := go_metrics.NewRegistry()
	log := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	_, err := NewSink(log, registry, cfg)
	require.NoError(err)

	// Verify configuration is properly stored
	assert.Equal("SASL_SSL", cfg.SecurityProtocol)
	assert.Equal("GSSAPI", cfg.SASLMechanism)
	assert.Equal("kafka", cfg.KerberosServiceName)
	assert.Equal("EXAMPLE.COM", cfg.KerberosRealm)
	assert.Equal("ktranslate@EXAMPLE.COM", cfg.KerberosPrincipal)
	assert.Equal("/etc/keytabs/ktranslate.keytab", cfg.KerberosKeytabPath)
	assert.Equal("/etc/krb5.conf", cfg.KerberosConfigPath)
	assert.Equal(false, cfg.KerberosDisablePAFXFAST)
	assert.Equal(-1, cfg.RequiredAcks)
	assert.Equal("snappy", cfg.Compression)
}

func TestKafkaSinkSSLConfiguration(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Test SSL configuration
	cfg := &ktranslate.KafkaSinkConfig{
		Topic:            "test-topic",
		BootstrapServers: "localhost:9093",
		SecurityProtocol: "SSL",
		SSLCAFile:        "/etc/ssl/ca-cert.pem",
		SSLCertFile:      "/etc/ssl/client-cert.pem",
		SSLKeyFile:       "/etc/ssl/client-key.pem",
		SSLKeyPassword:   "keypassword",
		SSLInsecure:      false,
	}

	registry := go_metrics.NewRegistry()
	log := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	_, err := NewSink(log, registry, cfg)
	require.NoError(err)

	// Verify SSL configuration is properly stored
	assert.Equal("SSL", cfg.SecurityProtocol)
	assert.Equal("/etc/ssl/ca-cert.pem", cfg.SSLCAFile)
	assert.Equal("/etc/ssl/client-cert.pem", cfg.SSLCertFile)
	assert.Equal("/etc/ssl/client-key.pem", cfg.SSLKeyFile)
	assert.Equal("keypassword", cfg.SSLKeyPassword)
	assert.Equal(false, cfg.SSLInsecure)
}

func TestKafkaSinkSASLMechanisms(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	testCases := []struct {
		mechanism string
		username  string
		password  string
	}{
		{"PLAIN", "testuser", "testpass"},
		{"SCRAM-SHA-256", "testuser", "testpass"},
		{"SCRAM-SHA-512", "testuser", "testpass"},
	}

	for _, tc := range testCases {
		cfg := &ktranslate.KafkaSinkConfig{
			Topic:            "test-topic",
			BootstrapServers: "localhost:9092",
			SecurityProtocol: "SASL_PLAINTEXT",
			SASLMechanism:    tc.mechanism,
			SASLUsername:     tc.username,
			SASLPassword:     tc.password,
		}

		registry := go_metrics.NewRegistry()
		log := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

		_, err := NewSink(log, registry, cfg)
		require.NoError(err)

		assert.Equal(tc.mechanism, cfg.SASLMechanism)
		assert.Equal(tc.username, cfg.SASLUsername)
		assert.Equal(tc.password, cfg.SASLPassword)
	}
}

func TestKafkaSinkCompressionTypes(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	compressionTypes := []string{"none", "gzip", "snappy", "lz4", "zstd"}

	for _, compression := range compressionTypes {
		cfg := &ktranslate.KafkaSinkConfig{
			Topic:            "test-topic",
			BootstrapServers: "localhost:9092",
			SecurityProtocol: "PLAINTEXT",
			Compression:      compression,
		}

		registry := go_metrics.NewRegistry()
		log := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

		_, err := NewSink(log, registry, cfg)
		require.NoError(err)
		assert.Equal(compression, cfg.Compression)
	}
}

func TestKafkaSinkDefaultValues(t *testing.T) {
	assert := assert.New(t)

	// Test default configuration values
	cfg := &ktranslate.KafkaSinkConfig{}
	*cfg = *ktranslate.DefaultConfig().KafkaSink

	assert.Equal("", cfg.Topic)
	assert.Equal("", cfg.BootstrapServers)
	assert.Equal("PLAINTEXT", cfg.SecurityProtocol)
	assert.Equal("", cfg.SASLMechanism)
	assert.Equal("kafka", cfg.KerberosServiceName)
	assert.Equal("/etc/krb5.conf", cfg.KerberosConfigPath)
	assert.Equal(false, cfg.KerberosDisablePAFXFAST)
	assert.Equal(false, cfg.SSLInsecure)
	assert.Equal(1, cfg.RequiredAcks)
	assert.Equal("none", cfg.Compression)
	assert.Equal(1000000, cfg.MaxMessageBytes)
	assert.Equal(3, cfg.RetryMax)
	assert.Equal(100, cfg.FlushFrequency)
	assert.Equal(100, cfg.FlushMessages)
	assert.Equal(64*1024, cfg.FlushBytes)
}
