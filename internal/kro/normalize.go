package kro

import "strings"

func normalizeControllerResource(tpl map[string]any) {
	stripCreationTimestamp(tpl)
	enforceManagedByLabel(tpl)

	kind, _ := tpl["kind"].(string)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "serviceaccount":
		normalizeServiceAccount(tpl)
	case "deployment":
		normalizeDeployment(tpl)
	}
}

func stripCreationTimestamp(node any) {
	switch val := node.(type) {
	case map[string]any:
		if meta, ok := val["metadata"].(map[string]any); ok {
			delete(meta, "creationTimestamp")
		}
		for _, child := range val {
			stripCreationTimestamp(child)
		}
	case []any:
		for _, child := range val {
			stripCreationTimestamp(child)
		}
	}
}

func enforceManagedByLabel(node any) {
	switch val := node.(type) {
	case map[string]any:
		if labels, ok := val["labels"].(map[string]any); ok {
			if _, exists := labels["app.kubernetes.io/managed-by"]; exists {
				labels["app.kubernetes.io/managed-by"] = "Kro"
			}
		}
		for _, child := range val {
			enforceManagedByLabel(child)
		}
	case []any:
		for _, child := range val {
			enforceManagedByLabel(child)
		}
	}
}

func normalizeServiceAccount(tpl map[string]any) {
	meta := ensureMap(tpl, "metadata")
	meta["name"] = "_SA_NAME_"
	meta["namespace"] = "_NAMESPACE_"
	meta["annotations"] = "_SA_ANNOTATIONS_"
}

func normalizeDeployment(tpl map[string]any) {
	meta := ensureMap(tpl, "metadata")
	meta["annotations"] = "_DEP_ANNOTATIONS_"
	meta["namespace"] = "_NAMESPACE_"

	spec := ensureMap(tpl, "spec")
	spec["replicas"] = "_DEP_REPLICAS_"

	template := ensureMap(spec, "template")
	tmplMeta := ensureMap(template, "metadata")
	tmplMeta["annotations"] = "_DEP_ANNOTATIONS_"

	podSpec := ensureMap(template, "spec")
	podSpec["serviceAccountName"] = "_SA_NAME_"

	normalizeContainers(podSpec)
}

func normalizeContainers(podSpec map[string]any) {
	containers, ok := podSpec["containers"].([]any)
	if !ok {
		return
	}
	for i, c := range containers {
		container, ok := c.(map[string]any)
		if !ok {
			continue
		}
		normalizeControllerContainer(container)
		containers[i] = container
	}
	podSpec["containers"] = containers
}

func normalizeControllerContainer(container map[string]any) {
	container["args"] = []any{
		"--aws-region", "_AWS_REGION_",
		"--aws-endpoint-url", "_AWS_ENDPOINT_URL_",
		"--log-level", "_LOG_LEVEL_",
		"--resource-tags", "_RESOURCE_TAGS_",
		"--watch-namespace", "_WATCH_NAMESPACE_",
		"--watch-selectors", "_WATCH_SELECTORS_",
		"--reconcile-resources", "_RECONCILE_RESOURCES_",
		"--deletion-policy", "_DELETION_POLICY_",
		"--reconcile-default-resync-seconds", "_RECONCILE_DEFAULT_RESYNC_",
		"--reconcile-default-max-concurrent-syncs", "_RECONCILE_DEFAULT_MAX_CONC_",
		"--feature-gates", "_FEATURE_GATES_",
		"--enable-carm", "_ENABLE_CARM_",
	}

	env := []any{
		map[string]any{
			"name": "ACK_SYSTEM_NAMESPACE",
			"valueFrom": map[string]any{
				"fieldRef": map[string]any{
					"fieldPath": "metadata.namespace",
				},
			},
		},
		envValue("AWS_REGION", "_AWS_REGION_"),
		envValue("AWS_ENDPOINT_URL", "_AWS_ENDPOINT_URL_"),
		envValue("ACK_WATCH_NAMESPACE", "_WATCH_NAMESPACE_"),
		envValue("ACK_WATCH_SELECTORS", "_WATCH_SELECTORS_"),
		envValue("RECONCILE_RESOURCES", "_RECONCILE_RESOURCES_"),
		envValue("DELETION_POLICY", "_DELETION_POLICY_"),
		envValue("LEADER_ELECTION_NAMESPACE", "_LEADER_ELECTION_NAMESPACE_"),
		envValue("ACK_LOG_LEVEL", "_LOG_LEVEL_"),
		envValue("ACK_RESOURCE_TAGS", "_RESOURCE_TAGS_"),
		envValue("RECONCILE_DEFAULT_RESYNC_SECONDS", "_RECONCILE_DEFAULT_RESYNC_"),
		envValue("RECONCILE_DEFAULT_MAX_CONCURRENT_SYNCS", "_RECONCILE_DEFAULT_MAX_CONC_"),
		envValue("FEATURE_GATES", "_FEATURE_GATES_"),
	}
	container["env"] = env

	container["image"] = "_IMAGE_REPOSITORY_:_IMAGE_TAG_"
	container["imagePullPolicy"] = "_IMAGE_PULL_POLICY_"

	if ports, ok := container["ports"].([]any); ok && len(ports) > 0 {
		if port, ok := ports[0].(map[string]any); ok {
			port["containerPort"] = "_DEP_PORT_"
			ports[0] = port
		}
		container["ports"] = ports
	}

	container["resources"] = map[string]any{
		"limits": map[string]any{
			"cpu":    "_RES_LIMITS_CPU_",
			"memory": "_RES_LIMITS_MEMORY_",
		},
		"requests": map[string]any{
			"cpu":    "_RES_REQUESTS_CPU_",
			"memory": "_RES_REQUESTS_MEMORY_",
		},
	}
}

func envValue(name, sentinel string) map[string]any {
	return map[string]any{
		"name":  name,
		"value": sentinel,
	}
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return map[string]any{}
	}
	if child, ok := parent[key].(map[string]any); ok {
		return child
	}
	child := map[string]any{}
	parent[key] = child
	return child
}
