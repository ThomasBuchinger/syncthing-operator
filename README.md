# Syncthing Operator
Kubernetes Operator for [syncthing](https://syncthing.net/)


# Features
**Persistent configuration**: Manual configuration

We respect manual configurations. The configuration directory is stored on a persistent volume and the 
operator only configures what it is told to using the REST API.

**Multiple data voumes**: Want to store your pictures on a different device than the rest?

# Usage
## Installation
**Certificates**: Generate openssl certificates and make sure to use "syncthing" as common name.

**Kubernetes** Currently the only way to install the operator is to clone the project and `make deploy`. Make sure kubectl points to the correct cluster and namespace

## Instance Configuration
Syncthing Instance controlls the Application and global options. By default the WebUI has authentication tunred on with username `syncthing`
and password set to the apikey

```yaml
apiVersion: syncthing.buc.sh/v1alpha1
kind: Instance
metadata:
  # Syncthing will advertise its name as "<name>-<random-letters>" to other devices
  name: example-syncthing
spec:
  # tag of the official hub.docker.com/r/syncthing/syncthing image to use
  # Optional. Default: latest
  tag: latest

  # Persistent Volume to store configuration
  # If you want to keep manual configuration changes accros restarts, make sure this volume is persistent (e.g. using hostpath or NFS)
  # If you configure everything via the operator, just omit this option and the operator will use ephemeral storage.
  # Optional. Default EmptyDir
  config_volume:
    name: config-root
    hostPath:
      path: /tmp/syncthing-config

  # Persistent volume for data
  # NOTE: You need at least a volume named 'data-root'
  # data-root volume is mounted in /var/syncthing and is the default location for synchronized folders
  # Additional volumes are mounted in /var/syncthing/<volume-name> and can be used to store folders on different disks or devices
  # You can configure any volume type supported by kubernetes: https://kubernetes.io/docs/concepts/storage/volumes/
  data_volumes:
    - name: data-root
      hostPath:
        path: /tmp/syncthing-data

  # apikey for authenticating the Rest API. This is used as the initial password for the syncthing-user on the WebUI
  apikey: "this-is-my-very-secret-api-key"

  # TLS private Key and Cert. You need to generate these yourself or copy from an existing syncthing instance
  tls_key: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
  tls_crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----

```

## Device Configuration
Device Resource
```yaml
apiVersion: syncthing.buc.sh/v1alpha1
kind: Device
metadata:
  # Name of the device in Syncthing
  name: fedora34
spec:
  # Id of the device
  id: "KJ2KICY-7XOAWLI-4KO7SSL-6K2373Q-2QZLFJ5-LKS63UU-CMJPEOL-T7O5WQY"
  
  # Limit bandwidth used by this device
  max_send_speed: "1M"
  max_receive_speed: "1M"

```

## Folder Configuration
Folder Configuration

```yaml
apiVersion: syncthing.buc.sh/v1alpha1
kind: Folder
metadata:
  # folder id in syncthing. Must be unique within the cluster
  name: folder-sample
spec:
  # Display Name of the folder
  label: "MyTestFolder"
  path: /var/syncthing/test
  type: "sendonly"
  
  # Add shared devices by device name
  shared_devices:
  - "Nokia 3.4"
  # Add shared devices by their ID
  shared_ids:
  - "KJ2KICY-7XOAWLI-4KO7SSL-6K2373Q-2QZLFJ5-LKS63UU-CMJPEOL-T7O5WQY"

  ignore_pattern:
  - "not_this_file"
  ignore_permissions: false

```

# State of the Project
Currently this project exists to, 1. practice programming on go, 2. practice kubernetes operators and 3. for personal use. It is **not**
ready for production use.

# Known Limitations:
Currently known Limitations are:
* Only 1 Syncthing-Instance is supported per cluster, because the Nodeports are hardcoded
* There are no options to configure how the WebUI is exposed to the outside world
* Syncthing should probably run as a Stateful Set instead of a deployment

