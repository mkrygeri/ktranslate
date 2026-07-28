#!/bin/bash
# Generate a self-signed certificate for HAProxy with IP SANs
# Output: /etc/haproxy/certs/firehose.pem (combined cert + key)

set -e

CERT_DIR="/etc/haproxy/certs"
CERT_NAME="firehose"
DAYS=365

# SANs
IP1="73.143.190.212"
IP2="192.168.2.218"

echo "Creating certificate directory..."
sudo mkdir -p "$CERT_DIR"

# Create OpenSSL config with SANs
TMPCONF=$(mktemp)
cat > "$TMPCONF" << EOF
[req]
default_bits       = 2048
prompt             = no
default_md         = sha256
distinguished_name = dn
x509_extensions    = v3_req

[dn]
C  = US
ST = State
L  = City
O  = ktranslate
CN = ${IP1}

[v3_req]
subjectAltName = @alt_names
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth

[alt_names]
IP.1 = ${IP1}
IP.2 = ${IP2}
EOF

echo "Generating self-signed certificate..."
openssl req -x509 -nodes -newkey rsa:2048 \
  -keyout /tmp/${CERT_NAME}.key \
  -out /tmp/${CERT_NAME}.crt \
  -days "$DAYS" \
  -config "$TMPCONF"

# HAProxy requires cert + key in a single PEM file
echo "Combining cert and key for HAProxy..."
sudo bash -c "cat /tmp/${CERT_NAME}.crt /tmp/${CERT_NAME}.key > ${CERT_DIR}/${CERT_NAME}.pem"
sudo chmod 600 "${CERT_DIR}/${CERT_NAME}.pem"

# Cleanup
rm -f /tmp/${CERT_NAME}.key /tmp/${CERT_NAME}.crt "$TMPCONF"

echo "Done. Certificate created at ${CERT_DIR}/${CERT_NAME}.pem"
echo ""
echo "Verify with:"
echo "  openssl x509 -in ${CERT_DIR}/${CERT_NAME}.pem -text -noout | grep -A4 'Subject Alternative Name'"
echo ""
echo "Restart HAProxy:"
echo "  sudo systemctl restart haproxy"
