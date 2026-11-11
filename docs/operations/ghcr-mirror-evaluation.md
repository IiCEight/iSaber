# GHCR mirror evaluation

Date: November 11, 2026. Target: the existing Tencent server, using its direct connection without the local proxy. No Docker configuration, deployment, or backup schedule was changed.

| Check | DaoCloud | NJU |
| --- | --- | --- |
| Public `astral-sh/uv` image | Complete Docker pull succeeded | Complete Docker pull succeeded |
| Original manifest digest | Preserved | Preserved |
| Uncached transfer measured independently of Docker's cache | Not timed separately | 19,338,545 bytes of config/layers in about two seconds; every SHA-256 and size verified |
| `iiceight/isaber-frontend` | Explicitly rejected: repository not in allowlist | HTTP 404; the proposed image is not published, so this is inconclusive |
| Public image actually published under our account | Explicitly rejected by allowlist | Four versions pulled successfully; tags and digests matched CI, and marker contents matched |
| Versions published after the first successful mirror pulls | Ineligible | Both new versions passed; complete pull/digest/content checks took 14 and 10 seconds |

The public test image was pinned to `sha256:96f40d3904aad8d0bd010539001d36c8e2188b981e1334471a90c4a1d93bb490`. Its linux/amd64 child manifest, config, and both layers were fetched from NJU into a temporary directory, independently of the shared Docker cache, and hashed. The directory was removed after the test. This demonstrates a cold transfer on Tencent; it does not establish a cold upstream cache miss at NJU.

The downloaded image ran `uv --version` successfully as UID/GID 65532 with no network, no host mounts, a read-only filesystem, dropped capabilities, and bounded memory/processes. It reported uv 0.13.7. The test container removed itself; the existing RSSHub container remained running. Docker retains the small test image under both mirror references.

## Our own image test

Temporary [PR #25](https://github.com/IiCEight/iSaber/pull/25) ran a standalone probe workflow; it is not a release workflow and must not be merged. Successful [probe run 42968688080](https://github.com/IiCEight/iSaber/actions/runs/42968688080) published marker-only scratch images to `ghcr.io/iiceight/isaber-mirror-probe` using its ephemeral workflow token. No application source, credentials, or user content enters those images.

The local GitHub login has repository access but no Packages scopes; the package API returns HTTP 403. New personal GHCR packages default to private. The owner made [the disposable package](https://github.com/users/IiCEight/packages/container/package/isaber-mirror-probe) public. Tencent then pulled both first-attempt versions through NJU, checked the original published digest, and inspected the expected marker using temporary containers that were never started. These tag/digest checks took 22 and 26 seconds. The containers were removed immediately.

After those first successful mirror pulls, the workflow was rerun to publish two new versions with different contents. Both new tags and their exact digest references were pulled through NJU and their markers inspected successfully. The complete checks took 14 and 10 seconds. All requests were anonymous, with no daemon proxy configured; no upstream credentials were supplied to the mirror.

| Tag | CI-published manifest digest |
| --- | --- |
| `42968688080-1-v1` | `sha256:e1aebf0653eca00c125deca20ab53cbb11727566e10de0c567a3ab712ee91230` |
| `42968688080-1-v2` | `sha256:5dad8c6c18896d06184bb20f4737e9e15a7cf688dff17159a5e693ad3553fd8e` |
| `42968688080-2-v1` | `sha256:a9abe35814c12798a20bf08a57176d5e7c6f278e55af39da68b3f80da7bb9d46` |
| `42968688080-2-v2` | `sha256:6461d9a2530b4f21d8eb923b443d873f171961d41a1deeaa908e4b29b69fe675` |

The temporary publishing workflow is removed after evaluation and the draft PR closed without merging. The public marker-only package and small Docker image cache remain available for reproducing the checks.

## Decision so far

NJU passes this evaluation for public images from our own account, including fresh versions and original digests. DaoCloud's current allowlist prevents using it for our own application images. The proposed simplified route is GitHub Actions → GHCR → NJU mirror → the single blog host, using ordinary Docker pulls and no custom transport. A full frontend/backend image benchmark is still required before migrating production; tiny marker images and a popular cached upstream image do not establish full application cold-pull performance or long-term availability.

Sources: [DaoCloud usage](https://github.com/DaoCloud/public-image-mirror), [DaoCloud allowlist policy](https://github.com/DaoCloud/public-image-mirror/issues/2328), [GitHub package visibility](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility).
