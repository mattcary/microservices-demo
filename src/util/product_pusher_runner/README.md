## `product_pusher/`

This is a utility to push JSON product information (as defined in
`productcatalogservice/`) do a Cloud SQL instance.

## Running
Build & push the docker image, then use the yaml in kubernetes-manifests/ to
launch in a GKE cluster. By default the `productcatalogservice/` json catalog
will be attached to the container and pushed to the Cloud SQL instance that is
proxied in the sidecar.
