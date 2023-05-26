# Telemetry

The `telemetry.operator.kyma-project.io` CustomResourceDefinition (CRD) is a detailed description of the kind of data and the format used to define a telemetry module instance. To get the up-to-date CRD and show the output in the YAML format, run this command:

```bash
kubectl get crd telemetry.operator.kyma-project.io -o yaml
```

## Sample custom resource

The following Telemetry object defines a module`.

```yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Telemetry
metadata:
  name: default
Status:
  Conditions:
    Last Transition Time:  2023-05-23T14:57:11Z
    Message:               installation is ready and resources can be used
    Observed Generation:   2
    Reason:                Ready
    Status:                True
    Type:                  Installation
  State:                   Ready
```

For further examples, see the [samples](https://github.com/kyma-project/telemetry-manager/tree/main/config/samples) directory.

## Custom resource parameters

For details, see the [Telemetry specification file](https://github.com/kyma-project/telemetry-manager/blob/main/apis/operator/v1alpha1/telemetry_types.go).

<!-- The table below was generated automatically -->
<!-- Some special tags (html comments) are at the end of lines due to markdown requirements. -->
<!-- The content between "TABLE-START" and "TABLE-END" will be replaced -->

<!-- TABLE-START -->
### Telemetry.operator.kyma-project.io/v1alpha1

**Status:**

| Parameter | Type | Description |
| ---- | ----------- | ---- |
| **conditions**  | \[\]object | Conditions contain a set of conditionals to determine the State of Status. If all Conditions are met, State is expected to be in StateReady. |
| **conditions.&#x200b;lastTransitionTime** (required) | string | lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable. |
| **conditions.&#x200b;message** (required) | string | message is a human readable message indicating details about the transition. This may be an empty string. |
| **conditions.&#x200b;observedGeneration**  | integer | observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance. |
| **conditions.&#x200b;reason** (required) | string | reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty. |
| **conditions.&#x200b;status** (required) | string | status of the condition, one of True, False, Unknown. |
| **conditions.&#x200b;type** (required) | string | type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt) |
| **state** (required) | string | State signifies current state of Module CR. Value can be one of ("Ready", "Processing", "Error", "Deleting"). |

<!-- TABLE-END -->
