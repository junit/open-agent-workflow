# Machine Assurance

Machine Assurance is an optional component for deployments that need
machine-verifiable evidence. It is an overlay on the static Policy product,
not a second workflow definition.

It may verify Profile bytes, Skill identity, Host observations, or cooperating
client events. Its records must be secret-free and must state their scope.
Evidence can increase confidence in a claim but cannot decide whether a
Policy Profile exists and cannot veto a Policy-valid workflow.

The Agent Host remains responsible for physical execution and security policy.
Assurance failure is reported as missing evidence; the model can continue on
the normal Policy path unless the user explicitly requested an assurance-only
deliverable.

Keep schemas, digests, leases, and receipts inside the optional component.
Portable Policy and Profiles must remain readable and useful without them.
