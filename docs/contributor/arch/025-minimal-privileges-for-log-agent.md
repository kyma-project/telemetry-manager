---
title: "Minimal Privileges for Log Agent"
date: 2025-01-15
---


# 25. Minimal Privileges for Log Agent

## Context

The log-agent is a component that reads logs directly from the node filesystem and forwards it to a configured sink. It requires specific permissions to access the logs of pods in the cluster. The goal is to define the minimal set of privileges required for the log-agent to function correctly without granting excessive permissions.

## Current Situation

The log agent currently runs with elevated privileges, which allows it to access all Pod logs in the cluster. This is not ideal from a security perspective.
The log agent needs elevated privileges for the following tasks:

1. Maintain a file cache that is persistent across restarts, so it can resume reading logs from where it left off.
2. Read logs from all Pods, including those that are not in the same namespace as the log agent.

## Possible Solutions

### Cache Directory Permissions
The log agent must be able to write to a filesystem location that is accessible to it. Because our log agent runs as a DaemonSet, it has access to the host filesystem. 

### Option a
The log agent can use a directory in `/var/lib/` for its file cache, which by default is **not** writeable by the DaemonSet user. We can grant write access to this directory with an init container that runs chmod on the directory before the log agent starts. To allow changing the permissions, the init container must run with elevated privileges, which is acceptable because it only runs once during the Pod startup.

### Option b
Alternatively, we can store those cache entries in a subdirectory of `/var/tmp/`, which by default is writable by the DaemonSet user. This would not require an init container. To circumvent the issue that the log agent cannot write to the folder, the directory must be created by the log agent itself (instead of by the kubelet at startup). For this, we can use the `create_directory` option of the `file_storage` extension.

Telemetry log agent change:
```yaml
...
    extensions:
        file_storage:
            create_directory: true
            directory: /var/lib/telemetry-log-agent/file-log-receiver
...
```

### Log Reading Permissions

The log agent needs to read logs from all Pods in the cluster. Those logs are stored in the `/var/log/pods` on the Node filesystem. The log agent needs read access to this directory. The default permissions of this directory allow access only to the root user and root group. To allow the log agent to read the logs, we have the following options:

### Option a
Grant read access to the `/var/log/pods` directory to the log agent user. This can be done by running the log agent with the `runAsGroup: 0` security context option in the Pod spec.

### Option b
Add additional capabilities to the log agent Pod, so that it can read the logs without changing the permissions of the `/var/log/pods` directory. This can be done by adding the `CAP_DAC_READ_SEARCH` capability to the log agent Pod. With this capability, the log agent can read files and directories, regardless of the permissions set on them.
Addint this capability on the pod spec alone will not be sufficient as those capabilities are only added to the capability boundary set of the process. One way to actually grant those capabilities to the effective set of is to run setcap on the binary during image creation. This can be done by adding the following line to the Dockerfile of the log-agent:

```dockerfile
RUN setcap cap_dac_read_search+ep /usr/local/bin/telemetry-log-agent
```

## Decision

For the cache directory, we will go with **Option b** and use the `file_storage` extension to create the directory in `/var/tmp/telemetry-log-agent/file-log-receiver`. This avoids the need for an init container and simplifies the deployment.
For the log reading permissions, we choose **Option a** and run the log agent with security context option `runAsGroup: 0`. This allows the log agent to read the logs without changing the permissions of the `/var/log/pods` directory.

The final log agent configuration will look like this:

```yaml
    spec:
    ...
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          privileged: false
          readOnlyRootFilesystem: true
          runAsGroup: 0
          runAsNonRoot: true
          runAsUser: 1000
          seccompProfile:
            type: RuntimeDefault
    ...
        volumeMounts:
        - mountPath: /var/lib/telemetry-log-agent
          name: varlibfilelogreceiver
    ...
      volumes:
      - hostPath:
          path: /var/tmp
          type: DirectoryOrCreate
        name: varlibfilelogreceiver
```


