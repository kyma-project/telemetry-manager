# Deploying `ModuleTemplate` with the Lifecycle Manager


1. Bundle the `telemetry` module as an OCI image and pusht it to a registery. You can push the OCI image either to a local k3d registry or to a remote registry.
    
    *   Using a local k3d registry:
        
        > **NOTE:** The following sub-steps assume that you have created a k3d cluster using the Kyma CLI with `kyma provision k3d`. This command creates a k3d registry which is exposed on port `5001`.
        
        *   Create the telemetry module, bundle it as an OCI image and push it to the local k3d registry:
            ```shell
            kyma alpha create module --name kyma-project.io/module/telemetry --version 0.0.1 --channel alpha --default-cr ./config/samples/operator_v1alpha1_telemetry.yaml --registry localhost:5001 --insecure
            ```
            The auto-generated `template.yaml` file contains the `ModuleTemplate`.

        *   In the `template.yaml` file, change the `spec.descriptor.component.repositoryContexts.baseUrl` field from `http://localhost:5001` to `http://k3d-kyma-registry.localhost:5000`. This allows the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main) to pull the module image from inside the k3d cluster.

    *   Using a remote registry:
        
        > **NOTE** In this example, we use the Google Container Registry (GCR) as our remote registry. The following sub-steps assume that you are a member of the Google Cloud Platform (GCP) project called `sap-kyma-huskies-dev`.

        *   In order to be able to create OAuth2 access tokens, ensure [here](https://console.cloud.google.com/iam-admin/iam?project=sap-kyma-huskies-dev) that you are assigned the `Service Account Token Creator` role.

        *   Authorize `gcloud` to access the Cloud Platform with Google user credentials:
            ```shell
            gcloud auth login
            ```
        
        *   Set the `project` property to `sap-kyma-huskies-dev`:
            ```shell
            gcloud config set project sap-kyma-huskies-dev
            ```
        
        * Set an environment variable (`GCR_TOKEN`) with the access token for the Service Account calld `telemetry-module-sa`:
             ```shell
            export GCR_TOKEN=$(gcloud auth print-access-token --impersonate-service-account telemetry-module-sa@sap-kyma-huskies-dev.iam.gserviceaccount.com)
            ```

        * Create the telemetry module, bundle it as an OCI image and push it to the GCR registry:
             ```shell
            kyma alpha create module --name kyma-project.io/module/telemetry --version 0.0.1 --channel alpha --default-cr ./config/samples/operator_v1alpha1_telemetry.yaml --registry https://europe-west3-docker.pkg.dev/sap-kyma-huskies-dev/telemetry-module --credentials oauth2accesstoken:$GCR_TOKEN
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