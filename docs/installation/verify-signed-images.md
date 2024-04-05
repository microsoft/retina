# Verify signed images

Retina images published to GHCR are cryptographically signed. You can verify their provenance with [`sigstore/cosign`](https://github.com/sigstore/cosign):

```shell
REPO=microsoft/retina # or your repo
IMAGE=retina-operator # or other image to verify
TAG=v0.0.6 # or other tag to verify OR replace with the image SHA256
cosign verify ghcr.io/$REPO/$IMAGE:$TAG --certificate-oidc-issuer https://token.actions.githubusercontent.com --certificate-identity-regexp="https://github.com/$REPO" -o text
```