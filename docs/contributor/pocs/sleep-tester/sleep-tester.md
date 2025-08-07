# Sleep Tester PoC

This proof of concept demonstrates the minimum possible sleep time for a GO application running on Linux in a Kubernetes cluster.

## Steps for Demonstration

1. Create a k3d cluster
  ```bash
  k3d cluster create kyma
  ```

2. Deploy the `sleep-tester` Pod
  ```bash
  kubectl apply -f sleep-tester.yaml
  ```

3. Wait until the status of the `sleep-tester` Pod is `Completed`

4. Check the logs of the `sleep-tester` Pod
  ```bash
    ➜  kubectl logs sleep-tester
    Slept: 967.834µs
    Slept: 2.179333ms
    Slept: 1.993458ms
    Slept: 2.019375ms
    Slept: 1.981792ms
    Slept: 1.965542ms
    Slept: 1.991041ms
    Slept: 2.002375ms
    Slept: 1.98525ms
    Slept: 1.99925ms
    Slept: 1.991042ms
    Slept: 1.987833ms
    Slept: 1.984583ms
    Slept: 2.019041ms
    Slept: 1.98325ms
    Slept: 1.969542ms
    Slept: 1.9955ms
    Slept: 1.999416ms
    Slept: 1.987334ms
    Slept: 2.000292ms
    Slept: 1.987292ms
    Slept: 2.000833ms
    Slept: 1.971291ms
    Slept: 2.008625ms
    Slept: 1.991208ms
    Slept: 1.986167ms
    Slept: 1.994959ms
    Slept: 1.99075ms
    Slept: 1.994084ms
    Slept: 1.990166ms
    Slept: 1.992541ms
    Slept: 1.992875ms
    Slept: 1.987875ms
    Slept: 2.003916ms
    Slept: 1.988958ms
    Slept: 1.988166ms
    Slept: 1.998375ms
    Slept: 1.991125ms
    Slept: 1.824125ms
    Slept: 2.145042ms
    Slept: 1.861916ms
    Slept: 2.125667ms
    Slept: 2.013125ms
    Slept: 1.980292ms
    Slept: 1.986417ms
    Slept: 1.9875ms
    Slept: 1.993334ms
    Slept: 1.990416ms
    Slept: 1.928833ms
    Slept: 2.056792ms
    Slept: 1.821542ms
    Slept: 2.154709ms
    Slept: 2.002ms
    Slept: 1.989375ms
    Slept: 2.02225ms
    Slept: 1.965209ms
    Slept: 2.021666ms
    Slept: 1.959333ms
    Slept: 1.988667ms
    Slept: 1.998458ms
    Slept: 1.98725ms
    Slept: 1.987375ms
    Slept: 1.982916ms
    Slept: 1.996375ms
    Slept: 1.990125ms
    Slept: 1.979958ms
    Slept: 1.993ms
    Slept: 1.993541ms
    Slept: 1.994209ms
    Slept: 1.9895ms
    Slept: 1.993584ms
    Slept: 1.98175ms
    Slept: 1.861875ms
    Slept: 2.114417ms
    Slept: 1.998417ms
    Slept: 1.992792ms
    Slept: 1.932666ms
    Slept: 2.01675ms
    Slept: 1.863375ms
    Slept: 1.990042ms
    Slept: 2.008ms
    Slept: 1.96225ms
    Slept: 1.771792ms
    Slept: 1.929291ms
    Slept: 1.972375ms
    Slept: 1.97225ms
    Slept: 1.966167ms
    Slept: 2.007958ms
    Slept: 2.045ms
    Slept: 1.965834ms
    Slept: 2.070334ms
    Slept: 1.9865ms
    Slept: 2.00325ms
    Slept: 1.982166ms
    Slept: 1.994375ms
    Slept: 1.999125ms
    Slept: 1.959792ms
    Slept: 1.96525ms
    Slept: 2.017291ms
    Slept: 1.970916ms

    Average sleep time: 1.978687ms
  ```

You can observe that although the `duration` flag is set to be `300µs`, the average sleep time was `1.978687ms`. This shows that the minimum sleep time possible for a GO application running on Linux in a Kubernetes cluster is around `2ms`.
