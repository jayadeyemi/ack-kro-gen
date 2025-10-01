package placeholders

// Defaults is a typed view if you need programmatic access elsewhere.
// The replacement engine still uses SchemaDefaults map.
type Defaults struct {
	Name        string
	Namespace   string
	AWS         struct {
		AccountID   string
		Region      string
		EndpointURL string
		Credentials struct {
			SecretName string
			SecretKey  string
			Profile    string
		}
	}
	DeletionPolicy string
	Image struct {
		Repository string
		Tag        string
		PullPolicy string
		PullSecrets []string
	}
	Deployment struct {
		Replicas           int
		ContainerPort      int
		Labels             map[string]any
		Annotations        map[string]any
		NodeSelector       map[string]string
		Tolerations        []any
		Affinity           map[string]any
		PriorityClassName  string
		HostNetwork        bool
		DNSPolicy          string
		Strategy           map[string]any
		ExtraVolumes       []any
		ExtraVolumeMounts  []any
		ExtraEnvVars       []any
	}
	Resources struct {
		Requests struct{ Memory, CPU string }
		Limits   struct{ Memory, CPU string }
	}
	Role struct{ Labels map[string]any }
	Metrics struct {
		Service struct {
			Create bool
			Type   string
		}
	}
	Log struct {
		EnableDev bool
		Level     string
	}
	InstallScope   string
	WatchNamespace string
	WatchSelectors string
	ResourceTags   []string
	Reconcile struct {
		DefaultResyncSeconds        int
		ResourceResyncSecondsByKind map[string]any
		DefaultMaxConcurrent        int
		ResourceMaxConcurrentByKind map[string]any
		Resources                   []string
	}
	EnableCARM   bool
	FeatureGates map[string]any
	ServiceAccount struct {
		Create      bool
		Name        string
		Annotations map[string]any
	}
	LeaderElection struct {
		Enabled   bool
		Namespace string
	}
	IAMRole struct {
		OIDCProvider       string
		MaxSessionDuration int
		Description        string
	}
}
