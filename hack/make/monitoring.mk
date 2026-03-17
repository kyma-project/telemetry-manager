##@ Monitoring

MONITORING_NAMESPACE ?= monitoring

.PHONY: deploy-monitoring
deploy-monitoring: $(HELM) ## Deploy Prometheus and Grafana into the monitoring namespace
	kubectl create ns $(MONITORING_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	$(HELM) repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
	$(HELM) repo add grafana https://grafana.github.io/helm-charts --force-update
	$(HELM) repo update
	$(HELM) upgrade --install prometheus prometheus-community/prometheus \
		-n $(MONITORING_NAMESPACE) \
		-f hack/monitoring/prometheus-values.yaml \
		--wait
	$(HELM) upgrade --install grafana grafana/grafana \
		-n $(MONITORING_NAMESPACE) \
		-f hack/monitoring/grafana-values.yaml \
		--wait
	@echo ""
	@echo "Monitoring deployed to namespace '$(MONITORING_NAMESPACE)'"
	@echo "  Grafana:    kubectl port-forward -n $(MONITORING_NAMESPACE) svc/grafana 3000:80"
	@echo "  Prometheus: kubectl port-forward -n $(MONITORING_NAMESPACE) svc/prometheus-server 9090:80"

.PHONY: undeploy-monitoring
undeploy-monitoring: $(HELM) ## Remove Prometheus and Grafana from the monitoring namespace
	$(HELM) uninstall grafana -n $(MONITORING_NAMESPACE) --ignore-not-found
	$(HELM) uninstall prometheus -n $(MONITORING_NAMESPACE) --ignore-not-found
	kubectl delete ns $(MONITORING_NAMESPACE) --ignore-not-found
