#!/bin/bash
# Setup script for KDC container

# Create and start a new KDC container
docker run -d --name ktranslate-kdc \
  -p 88:88/tcp \
  -p 88:88/udp \
  -p 464:464/tcp \
  -p 464:464/udp \
  ubuntu:20.04 \
  bash -c '
    # Update and install packages non-interactively
    export DEBIAN_FRONTEND=noninteractive
    apt-get update && apt-get install -y \
      krb5-kdc \
      krb5-admin-server \
      krb5-config \
      krb5-user \
      netcat \
      net-tools

    # Create Kerberos configuration
    cat > /etc/krb5.conf << EOF
[logging]
    default = FILE:/var/log/krb5libs.log
    kdc = FILE:/var/log/krb5kdc.log
    admin_server = FILE:/var/log/kadmind.log

[libdefaults]
    default_realm = EXAMPLE.COM
    kdc_timesync = 1
    ccache_type = 4
    forwardable = true
    proxiable = true
    fcc-mit-ticketflags = true
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ticket_lifetime = 24h
    renew_lifetime = 7d

[realms]
    EXAMPLE.COM = {
        kdc = localhost:88
        admin_server = localhost:749
        default_domain = example.com
    }

[domain_realm]
    .example.com = EXAMPLE.COM
    example.com = EXAMPLE.COM
EOF

    # Create the database
    echo "Creating Kerberos database..."
    kdb5_util create -s -r EXAMPLE.COM -P masterpass

    # Add principals
    echo "Adding principals..."
    kadmin.local -q "addprinc -pw ktranslatepass ktranslate@EXAMPLE.COM"
    kadmin.local -q "addprinc -pw kafkapass kafka/localhost@EXAMPLE.COM"
    
    # Create keytab
    echo "Creating keytab..."
    kadmin.local -q "ktadd -k /tmp/ktranslate.keytab ktranslate@EXAMPLE.COM"
    kadmin.local -q "ktadd -k /tmp/kafka.keytab kafka/localhost@EXAMPLE.COM"
    
    # Set proper permissions
    chmod 644 /tmp/*.keytab
    
    # Start services
    echo "Starting KDC services..."
    service krb5-kdc start
    service krb5-admin-server start
    
    echo "KDC setup complete!"
    echo "Available keytabs:"
    ls -la /tmp/*.keytab
    
    # Keep container running
    tail -f /var/log/krb5kdc.log /var/log/kadmind.log
  '