# SLSA 3 Container Build Example

## Introduction

This repository contains a template for publishing an
multi-arch (amd64,arm64) open source container image
containing a single static binary,
written in golang,
where the container is digitally signed,
and associated
[SLSA Level 3](https://slsa.dev/spec/v1.0/levels#build-l3)
provenance information,
also digitally signed,
is included.
Due to the use of the 
[SLSA Project GitHub Generators](https://github.com/slsa-framework/slsa-github-generator),
this template is only usable by public GitHub repositories
in non-restricted GitHub organizations.

The container produced by this template can be utilized
by a suitable Kubernetes admission controller such as
[Kyverno](https://kyverno.io/),
[Ratify](https://ratify.dev/), or
[Sigstore Kubernetes Policy Controller](https://docs.sigstore.dev/policy-controller/overview/)
to control which containers are deployable
by an administrator-defined policy on
container signatures or provenance information.

This template is shipped without a `Makefile`
so as to place focus on the github actions.

## Signing Standards Supported

The following container image signing standards are supported:

| Standard | Supported |
| ---      | ---       |
| Notary v1 (Docker Content Trust) | No  |
| Notary v2                        | No  |
| Sigstore Cosign                  | Yes - Keyless signing only |

## Template Features

### Sample Service

The sample service is a simple prometheus metrics exporter written in golang.
The exporter generates a `bank_account_balance` floating point metric
with a value between 0 and 100.
By default, the exporter listens on port 8081 and exports a `/metrics` endpoint.
The container's `ENTRYPOINT` is set to this binary.

### Local Build

The template contains a `.vscode` folder that configures the workspace
to use the `golangci-lint` linter
and creates a launch configuration for the sample binary.

### GitHub Actions

The GitHub actions are the main feature of this template.

#### SLSA L3 Go Builder Workflow

The `go-builder.yml` workflow is an implementation of the
[SLSA L3 Go Builder](https://github.com/slsa-framework/slsa-github-generator/blob/main/internal/builders/go/README.md).
The feature of this builder is that it generates a Go binary
with detailed signed provenance regarding how the binary was produced.
This build is special in that the binary and the provenance
are produced via independent jobs,
meaning that the build cannot cause the provenance to be falsified.
In addition to information about the GitHub workflow,
this builder also records compilation flags passed to the Go compiler.
When triggered, this builder performs the compile
and uploads the binary and provenance to the GitHub release area.

#### SLSA L3 Container Provenance-Only Workflow

The `container-generator.yml` workflow is an implementation of the
[SLSA L3 Container Provenance Generator](https://github.com/slsa-framework/slsa-github-generator/blob/main/internal/builders/container/README.md)
The feature of this builder is that it is an extension of the
`docker/build-push-action` that not only outputs a signed container image,
but also produces signed provenance about the built container.
As mentioned in the introduction,
the intention of this builder is the produce a container
with strong software supply-chain assurance.

In addition to provenance information,
the container workflow also runs the
[Trivy SBOM generator](https://aquasecurity.github.io/trivy/v0.48/docs/target/container_image/#generation)
on the container image.
The SBOM will capture OS packages from the base image,
as well as introspect the golang binary for its dependencies.

Tooling limitations prevent SBOM and SLSA attestations
from being included in the same attestation image.
For this reason, the deprecated
[SBOM attachments method](https://github.com/sigstore/cosign/blob/main/specs/SBOM_SPEC.md)
is used.

The workflow uploads an image index, whose hash is `(image-index-sha256)`.
The `cosign` container signature is uploaded as an image version
named `sha256-(image-index-sha256).sig` (for "signature").
The SLSA provenance and SBOM is uploaded as an image version
named `sha256-(image-index-sha256).att` (for "attestation").
The SBOM is uploaded as an image version
named `sha256-(image-index-sha256).sbom` (for "sbom"),
which itself has an accompanying `.sig` image.
The main, attestation, and sbom images are signed using `cosign` keyless signing.

#### OpenSSF Scorecard

This workflow is an implementation of the `ossf/scorecard-action`,
which creates and can publish security health metrics
for your open source project.


### Other Features

The template also has a placeholder `LICENSE` file
and [GitHub Dependabot support](https://github.com/dependabot).


## Policy Enforcement

Now, let's see how we can verify these images with 
[Kyverno](https://kyverno.io/),
[Ratify](https://ratify.dev/), or
[Sigstore Kubernetes Policy Controller](https://docs.sigstore.dev/policy-controller/overview/).

### Policy Enforcement with Kyverno

#### Signature Verification

We will use the [Kyverno sigstore image verifier](https://kyverno.io/docs/writing-policies/verify-images/sigstore/)
to check digital signatures on our container images.
Our example uses GitHub keyless signing via Sigstore Rekor.
Keyless signing and verification relieves both the producer and consumer
of the burden of cryptographic key management.

Here is an example cluster policy.
We are setting `verifyDigest: false` in this example
because our `kubectl run` will deploy by image tag instead of image hash,
which would be disallowed by the default `verifyImages` policy.

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-image-keyless
spec:
  validationFailureAction: Enforce
  webhookTimeoutSeconds: 30
  rules:
    - name: check-image-keyless
      match:
        any:
        - resources:
            kinds:
              - Pod
      verifyImages:
      - imageReferences:
        - ghcr.io/YOURORG/YOURCONTAINER:main
        verifyDigest: false
        attestors:
        - entries:
          - keyless:
              subject: https://github.com/YOURORG/YOURCONTAINER/.github/workflows/container-generator.yml@refs/heads/main
              issuer: https://token.actions.githubusercontent.com
              rekor:
                url: https://rekor.sigstore.dev
```

Next, try to launch your image

```shell
$ kubectl run myimage --image ghcr.io/YOURORG/YOURCONTAINER:main
pod/myimage created
```

To prove that we actually did something, edit the `subject` field in the policy
and make it intentionally wrong.
For example,

```
subject: https://github.com/YOURORG/SOMEOTHERCONTAINER/.github/workflows/container-generator.yml@refs/heads/main
```

Now, when you attempt to launch the image,
you will get an error:

```shell
$ kubectl run myimage --image ghcr.io/YOURORG/YOURCONTAINER:main
Error from server: admission webhook "mutate.kyverno.svc-fail" denied the request: 

resource Pod/default/myimage was blocked due to the following policies 

check-image-keyless:
  check-image-keyless: 'failed to verify image ghcr.io/YOURORG/YOURCONTAINER:main:
    .attestors[0].entries[0].keyless: subject mismatch: expected https://github.com/YOURORG/SOMEOTHERCONTAINER/.github/workflows/container-generator.yml@refs/heads/main,
    received https://github.com/YOURORG/YOURCONTAINER/.github/workflows/container-generator.yml@refs/heads/main'
```

Be sure to delete the pod when finished.

#### Provenance Verification (recommended)

We will use the [Kyverno SLSA provenance verifier](https://kyverno.io/policies/other/s-z/verify-image-slsa/verify-image-slsa/)
to to verify the SLSA provenance of our container images.
Container signature verification (above) only makes a statement
of who owns the signing key for the container:
it can sign the hash of any container.
Provenance verification goes a step further
by asserting that the container was built
from the indicated GitHub repository
at a given commit hash
using the indicated GitHub workflow.

Here is an example cluster policy.
As above, we are setting `verifyDigest: false` in this example
because our `kubectl run` will deploy by image tag instead of image hash.

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-image-provenance
spec:
  validationFailureAction: Enforce
  webhookTimeoutSeconds: 30
  rules:
    - name: check-image-keyless
      match:
        any:
        - resources:
            kinds:
              - Pod
      verifyImages:
      - imageReferences:
        - ghcr.io/YOURORG/YOURCONTAINER:main
        verifyDigest: false
        attestations:
        - predicateType: https://slsa.dev/provenance/v0.2
          attestors:
          - count: 1
            entries:
            - keyless:
                subject: "https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@refs/tags/v*"
                issuer: https://token.actions.githubusercontent.com
                rekor:
                  url: https://rekor.sigstore.dev
          conditions:
          - all:
            # This expression uses a regex pattern to ensure the builder.id in the attestation is equal to the official
            # SLSA provenance generator workflow and uses a tagged release in semver format. If using a specific SLSA
            # provenance generation workflow, you may need to adjust the first input as necessary.
            - key: "{{ regex_match('^https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@refs/tags/v[0-9].[0-9].[0-9]$','{{ builder.id }}') }}"
              operator: Equals
              value: true
           - key: "{{ invocation.configSource.uri }}"
              operator: Equals
              value: "git+https://github.com/YOURORG/YOURCONTAINER@*"
            - key: "{{ invocation.configSource.entryPoint }}"
              operator: Equals
              value: ".github/workflows/container-generator.yml"
```

This policy is slightly more complicated in the following ways:

* The `keyless.subject` (provenance signer) is the ID of the GitHub reusable workflow.
* The `builder.id` verification ensures that it really was the SLSA 3 container provenance workflow that was run.
* The `invocation.configSource.uri` ties the container to the GitHub repository that built it. (Without this, anyone who used the SLSA reusable workflow would be accepted by the policy!)
* The `invocation.configSource.entryPoint` ties the container to the specific workflow that built the container.

Similarly to container signing above, a successful launch:

```shell
$ kubectl run myimage --image ghcr.io/YOURORG/YOURCONTAINER:main
pod/myimage created
```

And an unsuccessful launch (one of the conditions fails, for example):

```shell
$ kubectl run myimage --image ghcr.io/YOURORG/YOURCONTAINER:main
Error from server: admission webhook "mutate.kyverno.svc-fail" denied the request: 

resource Pod/default/myimage was blocked due to the following policies 

check-image-provenance:
  check-image-keyless: '.attestations[0].attestors[0].entries[0].keyless: attestation
    checks failed for ghcr.io/YOURORG/YOURCONTAINER:main and predicate https://slsa.dev/provenance/v0.2: '
```

Be sure to delete the pod when finished.

### Policy Enforcement with Ratify

#### Signature Verification

Instructions to install Ratify as a Kubernetes Admission Controller
are on the [Ratify documentation site](https://ratify.dev/docs/1.0/quick-start).

Instructions to enable cosign keyless verification with Ratify are on
[GitHub](https://github.com/deislabs/ratify/blob/main/plugins/verifier/cosign/README.md#keyless-verification).

To enable keyless verification using `cosign`
update the `verifier/verifier-cosign` CRD
that is installed by Ratify.
The important change is to replace the
`key` parameter with the `rekorURL` parameter:

```yaml
apiVersion: config.ratify.deislabs.io/v1beta1
kind: Verifier
metadata:
  name: verifier-cosign
  annotations:
    helm.sh/hook: pre-install,pre-upgrade
    helm.sh/hook-weight: "5"
spec:
  name: cosign
  artifactTypes: application/vnd.dev.cosign.artifact.sig.v1+json
  parameters:
    rekorURL: https://rekor.sigstore.dev
```

The Ratify quickstart only enables verification on the `default` namespace.
To add additional namespaces,
update the `RatifyVerification/ratify-constraint` CRD:

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: RatifyVerification
metadata:
  name: ratify-constraint
spec:
  enforcementAction: deny
  match:
    kinds:
    - apiGroups:
      - ""
      kinds:
      - Pod
    namespaces:
    - default
```

Here is an example of a successful container launch:

```shell
$ kubectl run myimage -n default --image ghcr.io/YOURORG/YOURCONTAINER:main
pod/myimage created
```

And an unsuccessful one (at this time, `alpine` is not signed with `cosign`):

```shell
$ kubectl run myimage -n default --image alpine:latest
Error from server (Forbidden): admission webhook "validation.gatekeeper.sh" denied the request: [ratify-constraint] Subject failed verification: docker.io/library/alpine@sha256:eece025e432126ce23f223450a0326fbebde39cdf496a85d8c016293fc851978
```


There is also a command-line `ratify` tool to preview the results
outside of the Kubernetes admission controller.

Use a custom configuration file as follows:

```yaml
{
    "store": {
        "version": "1.0.0",
        "plugins": [
            {
                "name": "oras",
                "cosignEnabled": true
            }
        ]
    },
    "policy": {
        "version": "1.0.0",
        "plugin": {
            "name": "configPolicy",
            "artifactVerificationPolicies": {
                "application/vnd.dev.cosign.artifact.sig.v1+json": "any"
            }
        }
    },
    "verifier": {
        "version": "1.0.0",
        "plugins": [
            {
                "name": "cosign",
                "artifactTypes": "application/vnd.dev.cosign.artifact.sig.v1+json",
                "rekorURL": "https://rekor.sigstore.dev"
            }
        ]
    }
}
```

IMPORTANT!  Although the documentation says otherwise,
`rekorURL` is **required** to be specified in the `verifier`,
or keyless verification will FAIL.

An example of a failed verification:

```shell
$ ./ratify verify --config ~/.ratify/config --subject ghcr.io/YOURORG/YOURCONTAINER:main 
INFO[0000] Setting log level to info                    
Warning: Digest should be used instead of tagged reference. The resolved digest may not point to the same signed artifact, since tags are mutable.
INFO[0000] selected default auth provider: dockerConfig 
INFO[0000] defaultPluginPath set to /home/bnevis/.ratify/plugins 
INFO[0000] selected policy provider: configpolicy       
INFO[0000] Resolve of the image completed successfully the digest is sha256:0c7f3a284e31c23c42d2756b38db986b4605cf88b7dd1ecb1696e01b736a2ed9  component-type=executor go.version=go1.20.8
time="2023-11-03T19:16:30-07:00" level=info msg="selected default auth provider: dockerConfig"
{
  "verifierReports": [
    {
      "subject": "ghcr.io/YOURORG/YOURCONTAINER:main",
      "isSuccess": false,
      "name": "cosign",
      "message": "cosign verification failed: no valid signatures found",
      "extensions": {
        "signatures": [
          {
            "bundleVerified": false,
            "error": {},
            "isSuccess": false,
            "signatureDigest": "sha256:c4ac705f5d28feef4f535b048526b9d34739bfc24baed2bd1b12ca9e9a0c891b"
          }
        ]
      },
      "artifactType": "application/vnd.dev.cosign.artifact.sig.v1+json"
    }
  ]
}
```

And a successful verification:

```shell
$ ./ratify verify --config ~/.ratify/config --subject ghcr.io/YOURORG/YOURCONTAINER:main 
INFO[0000] Setting log level to info                    
Warning: Digest should be used instead of tagged reference. The resolved digest may not point to the same signed artifact, since tags are mutable.
INFO[0000] selected default auth provider: dockerConfig 
INFO[0000] defaultPluginPath set to /home/bnevis/.ratify/plugins 
INFO[0000] selected policy provider: configpolicy       
INFO[0000] Resolve of the image completed successfully the digest is sha256:0c7f3a284e31c23c42d2756b38db986b4605cf88b7dd1ecb1696e01b736a2ed9  component-type=executor go.version=go1.20.8
time="2023-11-05T18:28:59-08:00" level=info msg="selected default auth provider: dockerConfig"
{
  "isSuccess": true,
  "verifierReports": [
    {
      "subject": "ghcr.io/YOURORG/YOURCONTAINER:main",
      "isSuccess": true,
      "name": "cosign",
      "message": "cosign verification success. valid signatures found",
      "extensions": {
        "signatures": [
          {
            "bundleVerified": true,
            "isSuccess": true,
            "signatureDigest": "sha256:c4ac705f5d28feef4f535b048526b9d34739bfc24baed2bd1b12ca9e9a0c891b"
          }
        ]
      },
      "artifactType": "application/vnd.dev.cosign.artifact.sig.v1+json"
    }
  ]
}
```


### Policy Enforcement with Sigstore Kubernetes Policy Controller

#### Signature Verification

After using the [installation instructions](https://edu.chainguard.dev/open-source/sigstore/policy-controller/how-to-install-policy-controller/)
to install the policy-controller from the
[Sigstore helm charts](https://sigstore.github.io/helm-charts),
opt-in the namespace for policy enforcement:

```shell
$ kubectl label namespace default policy.sigstore.dev/include=true
```

Once opting in, deployments will fail unless a policy is defined.

```shell
$ kubectl run myimage --image ghcr.io/YOURORG/YOURCONTAINER:main
Error from server (BadRequest): admission webhook "policy.sigstore.dev" denied the request: validation failed: no matching policies: spec.containers[0].image
YOURORG/YOURCONTAINER/random_exporter@sha256:0c7f3a284e31c23c42d2756b38db986b4605cf88b7dd1ecb1696e01b736a2ed9
```

Install a sample policy:

```yaml
apiVersion: policy.sigstore.dev/v1alpha1
kind: ClusterImagePolicy
metadata:
  name: image-is-signed-by-github-actions
spec:
  images:
  # All images in example repository matched
  - glob: "ghcr.io/YOURORG/YOURIMAGE@sha256:*"
  authorities:
  - keyless:
      # Signed by the public Fulcio certificate authority
      url: https://fulcio.sigstore.dev
      identities:
      # Matches the Github Actions OIDC issuer
      - issuer: https://token.actions.githubusercontent.com
        # Matches a specific github workflow on main branch. Here we use the
        # sigstore policy controller example testing workflow as an example.
        subject: "https://github.com/YOURORG/YOURIMAGE/.github/workflows/container-generator.yml@refs/heads/main"
    ctlog:
      url: https://rekor.sigstore.dev
```

A correctly written policy will lead to a successful container launch:

```shell
$ kubectl run myimage --image ghcr.io/YOURORG/YOURCONTAINER:main
pod/myimage created
```

Be sure to delete the pod when finished.
