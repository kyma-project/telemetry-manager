# Deploying `ModuleTemplate` with the Lifecycle Manager


1. Bundle the `telemetry` module as an OCI image and pusht it to a registry. You can push the OCI image either to a local k3d registry or to a remote registry.
    
    *   Using a local k3d registry:
        
        > **NOTE:** The following sub-steps assume that you have created a k3d cluster using the Kyma CLI with `kyma provision k3d`. This command creates a k3d registry which is exposed on port `5001`.
        
        1. Create the `telemetry` module, bundle it as an OCI image and push it to the local k3d registry:
            ```shell
            kyma alpha create module --name kyma-project.io/module/telemetry --version 0.0.1 --channel alpha --default-cr ./config/samples/operator_v1alpha1_telemetry.yaml --registry localhost:5001 --insecure
            ```
            The auto-generated `template.yaml` file contains the `ModuleTemplate`.

        2. In the `template.yaml` file, change the `spec.descriptor.component.repositoryContexts.baseUrl` field from `http://localhost:5001` to `http://k3d-kyma-registry.localhost:5000`. Now the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main) can pull the module image from inside the k3d cluster.

    *   Using a remote registry:
        
        ```shell
        kyma alpha create module --name kyma-project.io/module/telemetry --version 0.0.1 --channel alpha --default-cr ./config/samples/operator_v1alpha1_telemetry.yaml --registry MODULE_REGISTRY --credentials USER:PASSWORD
        ``` 
          

2. Deploy the [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager/tree/main) to the cluster:

    ```shell
    kyma alpha deploy
    ```

3. Create the `ModuleTemplate` in the cluster:

    ```shell
    kubectl apply -f template.yaml
    ```

4. Verify that the `telemetry` module exists in the list of available modules:

    ```shell
    kyma alpha list module -A
    ```

5. Enable the `telemetry` module:

    ```shell
    kyma alpha enable module telemetry --channel alpha
    ```