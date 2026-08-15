package assets

import (
	"embed"
	"io/fs"
)

//go:embed schemas/v1/classification-proposal.schema.json schemas/v1/explicit-invocation-attestation.schema.json schemas/v1/gate-attestation.schema.json schemas/v1/profile-alias-set.schema.json schemas/v1/project-config.schema.json schemas/v1/user-authorization.schema.json schemas/v1/workflow-head.schema.json
//go:embed schemas/v2/dispatch-packet.schema.json schemas/v2/host-environment-report.schema.json schemas/v2/workflow-command.schema.json schemas/v2/workflow-result.schema.json schemas/v2/workflow-revision.schema.json schemas/v2/workflow-snapshot.schema.json
//go:embed schemas/v3/capability-grant.schema.json schemas/v3/host-binding-inventory.schema.json schemas/v3/host-integration-set.schema.json schemas/v3/host-integration.schema.json schemas/v3/host-invocation-receipt.schema.json schemas/v3/host-manifest.schema.json schemas/v3/host-session.schema.json schemas/v3/profile-recipe.schema.json schemas/v3/user-config.schema.json
//go:embed schemas/v4/execution-graph.schema.json schemas/v4/host-conformance-report.schema.json schemas/v4/host-conformance-transcript.schema.json schemas/v4/provider-descriptor.schema.json providers/*.json recipes/*.json audits/provider-sources-v4.json profile-aliases.json profile-matrix.json host-integrations.json
var embedded embed.FS

func FS() fs.FS { return embedded }
