# Image URL to use all building/pushing image targets;
# Use your own docker registry and image name for dev/test by overridding the
# IMAGE_REPO, DEVICE_IMAGE_NAME, WEBHOOK_IMAGE_NAME, PKG_IMAGE_NAME and IMAGE_TAG environment variable.
IMAGE_REPO ?= docker.io/bitfusiondeviceplugin
DEVICE_IMAGE_NAME ?= bitfusion-device-plugin
WEBHOOK_IMAGE_NAME ?= bitfusion-webhook
PKG_IMAGE_NAME ?= bitfusion-client
IMAGE_TAG  ?= 0.2.1
K8S_PLATFORM ?= community

export K8S_PLATFORM

IMAGE_TAG ?= $(shell date +v%Y%m%d)-$(shell git describe --match=$(git rev-parse --short=8 HEAD) --tags --always --dirty)

DEVICE_PLUGIN_ORIGINAL_NAME := phaedobf/device-plugin:v0.1
BF_CLIENT_ORIGINAL_NAME := phaedobf/bitfusion-client-ubuntu1804_2.5.0-10_amd64:v0.1
WEBHOOK_ORIGINAL_NAME := phaedobf/sidecar-injector:v0.1

DEVICE_PLUGIN_YAML_FILE := device-plugin/deployment/device_plugin.yml
CONFIGMAP_YAML_FILE := webhook/deployment/bitfusion-injector-webhook-configmap.yaml
WEBHOOK_YAML_FILE := webhook/deployment/bitfusion-injector.yaml

DEVICE_PLUGIN_YAML_TO_DEPLOY := device-plugin/deployment/deploy_device_plugin.yml
CONFIGMAP_YAML_TO_DEPLOY := webhook/deployment/deploy-bitfusion-injector-webhook-configmap.yaml
WEBHOOK_YAML_TO_DEPLOY := webhook/deployment/deploy-bitfusion-injector.yaml

PSP_YAML := device-plugin/deployment/privileged.yaml


############################################################
# image section
############################################################

image: build-image push-image

build-image:
	@echo "Building the docker image: $(IMAGE_REPO)/$(DEVICE_IMAGE_NAME):$(IMAGE_TAG)..."
	@docker build -t $(IMAGE_REPO)/$(DEVICE_IMAGE_NAME):$(IMAGE_TAG) -f device-plugin/build/Dockerfile .
	@echo "Building the docker image: $(IMAGE_REPO)/$(WEBHOOK_IMAGE_NAME):$(IMAGE_TAG)..."
	@docker build -t $(IMAGE_REPO)/$(WEBHOOK_IMAGE_NAME):$(IMAGE_TAG) -f webhook/build/Dockerfile .

push-image: build-image
	@echo "Pushing the docker image for $(IMAGE_REPO)/$(DEVICE_IMAGE_NAME):$(IMAGE_TAG) ..."
	@docker push $(IMAGE_REPO)/$(DEVICE_IMAGE_NAME):$(IMAGE_TAG)
	@echo "Pushing the docker image for $(IMAGE_REPO)/$(WEBHOOK_IMAGE_NAME):$(IMAGE_TAG) ..."
	@docker push $(IMAGE_REPO)/$(WEBHOOK_IMAGE_NAME):$(IMAGE_TAG)

############################################################
# deploy section
############################################################

deploy: update create

update:
	@echo "Create file $(DEVICE_PLUGIN_YAML_TO_DEPLOY) by $(DEVICE_PLUGIN_YAML_FILE): rewrite $(DEVICE_PLUGIN_ORIGINAL_NAME) to $(IMAGE_REPO)/$(DEVICE_IMAGE_NAME):$(IMAGE_TAG) ..."
	@cat $(DEVICE_PLUGIN_YAML_FILE) | sed -e "s|$(DEVICE_PLUGIN_ORIGINAL_NAME)|$(IMAGE_REPO)/$(DEVICE_IMAGE_NAME):$(IMAGE_TAG)|g" > $(DEVICE_PLUGIN_YAML_TO_DEPLOY)
	@echo "Create file $(CONFIGMAP_YAML_TO_DEPLOY) by $(CONFIGMAP_YAML_FILE): rewrite $(BF_CLIENT_ORIGINAL_NAME) to $(IMAGE_REPO)/$(PKG_IMAGE_NAME):$(IMAGE_TAG) ..."
	@cat $(CONFIGMAP_YAML_FILE) | sed -e "s|$(BF_CLIENT_ORIGINAL_NAME)|$(IMAGE_REPO)/$(PKG_IMAGE_NAME):$(IMAGE_TAG)|g" > $(CONFIGMAP_YAML_TO_DEPLOY)
	@echo "Create file $(WEBHOOK_YAML_TO_DEPLOY) by $(WEBHOOK_YAML_FILE): rewrite $(WEBHOOK_ORIGINAL_NAME) to $(IMAGE_REPO)/$(WEBHOOK_IMAGE_NAME):$(IMAGE_TAG) ..."
	@cat $(WEBHOOK_YAML_FILE) | sed -e "s|$(WEBHOOK_ORIGINAL_NAME)|$(IMAGE_REPO)/$(WEBHOOK_IMAGE_NAME):$(IMAGE_TAG)|g" > $(WEBHOOK_YAML_TO_DEPLOY)

create: update
ifeq ($(K8S_PLATFORM),tkgi)
	@echo "Create Pod Security Policy"
	-kubectl delete -f $(PSP_YAML)
	@kubectl create -f $(PSP_YAML)
	-kubectl -n kube-system delete role psp:privileged
	@kubectl -n kube-system create role psp:privileged \
    --verb=use \
    --resource=podsecuritypolicy \
    --resource-name=privileged
	@kubectl --as=system:serviceaccount:kube-system:default -n kube-system  auth can-i use podsecuritypolicy/privileged
endif
	@echo "Create device-plugin ..."
	-kubectl delete -f $(DEVICE_PLUGIN_YAML_TO_DEPLOY)
	@kubectl create -f $(DEVICE_PLUGIN_YAML_TO_DEPLOY)
	@echo "Create webhook ..."
	@bash webhook/deploy.sh


############################################################
# clear section
############################################################
uninstall: clear-object clear-file

clear-object:
	@echo "Delete wbehook ..."
	@kubectl delete -f webhook/deploy/deploy-bitfusion-injector.yaml
	@echo "Delete wbehook service ..."
	@kubectl delete -f webhook/deploy/bitfusion-injector-service.yaml
	@echo "Delete wbehook configmap ..."
	@kubectl delete -f webhook/deploy/deploy-bitfusion-injector-webhook-configmap.yaml
	@echo "Delete wbehook mutating configuration ..."
	@kubectl delete -f webhook/deploy/bitfusion-mutating-webhook-configuration.yaml
	@echo "Delete bwki service account ..."
	@kubectl delete -f webhook/deploy/bitfusion-service-account.yaml
	@echo "Delete wbehook validating configuration ..."
	@kubectl delete -f webhook/deploy/bitfusion-validating-webhook-configuration.yaml
	@echo "Delete namespace bwki ..."
	@kubectl delete ns bwki
	@echo "Delete device-plugin ..."
	@kubectl delete -f $(DEVICE_PLUGIN_YAML_TO_DEPLOY)

clear-file:
	@rm -rf webhook/deploy
	@rm -rf $(DEVICE_PLUGIN_YAML_TO_DEPLOY)
	@rm -rf $(CONFIGMAP_YAML_TO_DEPLOY)
	@rm -rf $(WEBHOOK_YAML_TO_DEPLOY)
