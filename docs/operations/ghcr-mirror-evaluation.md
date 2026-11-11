# GHCR mirror evaluation

Date: November 11, 2026. Target: the existing Tencent server, using its direct connection without the local proxy. No Docker configuration, deployment, or backup schedule was changed.

| Check | DaoCloud | NJU |
| --- | --- | --- |
| Public `astral-sh/uv` image | Complete Docker pull succeeded | Complete Docker pull succeeded |
| Original manifest digest | Preserved | Preserved |
| Uncached transfer measured independently of Docker's cache | Not timed separately | 19,338,545 bytes of config/layers in about two seconds; every SHA-256 and size verified |
| Full `iiceight` frontend/backend images | Our namespace is rejected by allowlist | Both native digest-pinned pulls succeeded; initial short limits timed out (details below) |
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

## Full application images

[CI run 42973863483](https://github.com/IiCEight/iSaber/actions/runs/42973863483) passed contract/frontend/backend checks, built both images once, exercised migrations/runtime-role permissions and real container acceptance, then pushed those existing local images to GHCR. The tag is `sha-130fd632f8447c569fa266c94fac4ac7dbcf9155` (the PR merge commit). Publication used a temporary allowance for this internal PR only; that allowance was removed before review. [Run 43006223325](https://github.com/IiCEight/iSaber/actions/runs/43006223325) passed all four checks with publication correctly skipped.

The owner reports both application packages are public. Anonymous native Docker pulls on Tencent confirmed this without any registry credentials, proxy change, or image archive transport. Neither full application image was present in Tencent's Docker cache before its first attempt.

| Image | Compressed layer bytes from manifest | Initial bounded attempt | Successful retry | Subsequent cached pull |
| --- | ---: | --- | ---: | ---: |
| Frontend | 95,722,572 | Timed out after 240 seconds on the 51,082,654-byte Node layer | 272.88 seconds | 0.28 seconds |
| Backend | 30,681,801 | Timed out after 180 seconds on the 30,553,299-byte binary layer | 212.46 seconds | 0.20 seconds |

Retry times exclude the initial attempts and intervening diagnostics; they are **not** fresh-cache end-to-end measurements. Earlier attempts retained completed layers. A second backend client joined the same daemon pull for its final 111.33 seconds; it is not another independent success. Registry compressed sizes are manifest metadata, not measured wire-byte counts. The sample does not establish long-term availability or a maximum cold-pull time.

Both Docker pulls and subsequent image inspection matched the original CI-published manifest digests:

- Frontend: `sha256:7a3fae43fba6180bdab7d9d1229760c17cf982327a95b873652578a22379359b`.
- Backend: `sha256:9b60683fbc7d6390ddcd6446acfb5948d6686776fadcf1eeb73bbb650e908101`.

The frontend ran only `node --version` in a temporary container with no network/host mounts, a read-only filesystem, dropped capabilities, and memory/process limits; it returned v24.22.2. Backend image inspection confirmed UID/GID 65532 and the expected API entrypoint; CI already exercised its real runtime. The temporary frontend container removed itself. RSSHub remained running. No blog stack, host files, DNS, Docker daemon configuration, timers, or article data were changed. Downloaded test/application/dependency images remain in the Docker cache.

A subsequent one-byte range request for the full Node layer returned HTTP 206 directly from NJU (`210.28.130.20`), with zero redirects. This confirms the tested cached large-layer request stays on NJU; it does not explain the earlier cold-transfer delay.

Official dependency pins were also tested using native Docker pulls. NJU's **Docker Hub** endpoint returned HTTP 403 on this host, so the Compose file uses DaoCloud's allowed official `docker.io/library` repositories: PostgreSQL digest `sha256:f9a7b9ebf6f56201f949db745e47bc423c0f0f8294bafe991cc10bf74b029657` pulled in 22.84 seconds; Caddy digest `sha256:fc9cbc100585871d18d4e451cca7021785c90a7006087427ae5ceb3e4efe6424` in 10.34 seconds. These are dependency pulls, separate from our GHCR application namespace.

## First merged main release

After PR #26 merged as `2440b19dfb4f11a48ae7d0b4586d15a0d2b17557`, [main CI 43011545816](https://github.com/IiCEight/iSaber/actions/runs/43011545816) passed all four checks and published the tested images. Native Tencent pulls through NJU completed without retry:

| Image | Main CI digest | Pull time |
| --- | --- | ---: |
| Frontend | `sha256:12a482b4dca475cf5f24d334286e2960aa34f40924590fc3bb40f341daae61b6` | 17.13 seconds |
| Backend | `sha256:b88638849b09d5804c8fe3565869f96a9726e4fe02b6596eff2abe5285d9a712` | 31.41 seconds |

Docker downloaded the changed application layers and reused existing base content. Both resulting repository digests matched CI. This is a new-release test with a populated host cache; it does not replace the initial cold-pull measurements. The application was subsequently installed on Tencent with preserved JD data and trusted public-IP HTTPS.

## Decision

Use ordinary Docker pulls through NJU for the public GHCR application images. This route has passed exact-digest full-image acceptance and cached repeats, alongside the earlier fresh-version probes. DaoCloud cannot serve our namespace, but can serve the pinned official database/proxy dependencies.

Keep cold-pull limitations explicit: pre-pull and verify all images during first host setup, before enabling delivery. Routine deployment bounds native Compose pulls at eight minutes and leaves current services running if pulling fails; a failed pull can be retried without a new transport framework. Docker reuses completed content. A cached pull of the same release is not evidence of a new release's transfer speed. Public IP HTTPS and the JD data cutover have passed. Automated main-to-Tencent release activation remains tracked in [deployment](deployment.md).

Sources: [DaoCloud usage](https://github.com/DaoCloud/public-image-mirror), [DaoCloud allowlist policy](https://github.com/DaoCloud/public-image-mirror/issues/2328), [GitHub package visibility](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility).
