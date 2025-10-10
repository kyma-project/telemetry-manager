# Telemetry Pipeline OTLP Output

The `otlp` output is the only output usually supported by a pipeline and is mandatory to configure. It will define where to push the telemetry data, what protocoll to use and which means of authentication to use.

![OTLP-Output](./../assets/otlp-output.drawio.svg)

## Basic configuration

The minimal configuration to define an output is to specify an OTLP endpoint without authentication using GRPC as underlying protocol.

```yaml
...
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

## Protocol

The default protocol for shipping the data to a backend is GRPC, but you can choose HTTP instead. Depending on the configured protocol, an `otlp` or an `otlphttp` exporter is used. Ensure that the correct port is configured as part of the endpoint.

- For GRPC, use:

  ```yaml
    ...
    output:
      otlp:
        endpoint:
          value: https://backend.example.com:4317
  ```

- For HTTP, use the `protocol` attribute:

  ```yaml
   ...
   output:
      otlp:
        protocol: http
        endpoint:
          value: https://backend.example.com:4318
  ```

## Authentication Details From Secrets

Integrations into external systems usually need authentication details dealing with sensitive data. To handle that data properly in Secrets, TracePipeline supports the reference of Secrets.

Using the **valueFrom** attribute, you can map Secret keys for mutual TLS (mTLS), Basic Authentication, or with custom headers.

You can store the value of the token in the referenced Secret without any prefix or scheme, and you can configure it in the `headers` section of the TracePipeline. In the following example, the token has the prefix "Bearer".

<!-- tabs:start -->

### **mTLS**

```yaml
  ...
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      tls:
        cert:
          valueFrom:
            secretKeyRef:
              name: backend
              namespace: default
              key: cert
        key:
          valueFrom:
            secretKeyRef:
              name: backend
              namespace: default
              key: key
```

### **Basic Authentication**

```yaml
  ...
  output:
    otlp:
      endpoint:
        valueFrom:
          secretKeyRef:
              name: backend
              namespace: default
              key: endpoint
      authentication:
        basic:
          user:
            valueFrom:
              secretKeyRef:
                name: backend
                namespace: default
                key: user
          password:
            valueFrom:
              secretKeyRef:
                name: backend
                namespace: default
                key: password
```

### **Token-based authentication with custom headers**

```yaml
  ...
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
      headers:
      - name: Authorization
        prefix: Bearer
        valueFrom:
          secretKeyRef:
              name: backend
              namespace: default
              key: token
```

<!-- tabs:end -->

The related Secret must have the referenced name, be located in the referenced namespace, and contain the mapped key. See the following example:

```yaml
kind: Secret
apiVersion: v1
metadata:
  name: backend
  namespace: default
stringData:
  endpoint: https://backend.example.com:4317
  user: myUser
  password: XXX
  token: YYY
  key: |
    -----BEGIN CERTIFICATE-----
    ...
  
  cert: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
```

### Secret Rotation

Telemetry Manager continuously watches the Secret referenced with the **secretKeyRef** construct. You can update the Secret’s values, and Telemetry Manager detects the changes and applies the new Secret to the setup.

> [!TIP]
> If you use a Secret owned by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator), you can configure an automated rotation using a `credentialsRotationPolicy` with a specific `rotationFrequency` and don’t have to intervene manually.

## Authentication Details From Plain Text

To integrate with external systems, you must configure authentication  details. You can use mutual TLS (mTLS), Basic Authentication, or custom headers:

<!-- tabs:start -->

### **mTLS**

```yaml
  ...
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      tls:
        cert:
          value: |
            -----BEGIN CERTIFICATE-----
            ...
        key:
          value: |
            -----BEGIN RSA PRIVATE KEY-----
            ...
```

### **Basic Authentication**

```yaml
  ...
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      authentication:
        basic:
          user:
            value: myUser
          password:
            value: myPwd
```

### **Token-based authentication with custom headers**

```yaml
  ...
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      headers:
      - name: Authorization
        prefix: Bearer
        value: "myToken"
```

<!-- tabs:end -->

## Istio Support

Communication to cluster-internal backends running in the Istio service mesh can leverage mTLS communication and with that improve the security of that communication channel.

The Telemetry module automatically detects whether the Istio module is added to your cluster, and injects Istio sidecars to the Telemetry components and will automatically support Istio mTLS if possible.

![Istio-Output](./../assets/istio-output.drawio.svg)
