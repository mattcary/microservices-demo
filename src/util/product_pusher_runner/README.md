## `product_pusher/`

This is a utility to push JSON product information (as defined in
`productcatalogservice/`) do a Cloud SQL instance.

## SQL Setup
The Cloud SQL instance must have a Catalog database with user and password wired
to the cloudsql-db-credentials secret. Database users can be managed through the
cloud sql instance in Pantheon. To create a `Catalog` DB accessible to a
`productcatalog@%` user, connect to the instance in cloud shell using `gcloud sql
connect` and run
```
> create database Catalog;
> grant all privileges on Catalog.* to 'productcatalog'@'%';
```

## Running
Build & push the docker image, then use the yaml in kubernetes-manifests/ to
launch in a GKE cluster. By default the `productcatalogservice/` json catalog
will be attached to the container and pushed to the Cloud SQL instance that is
proxied in the sidecar.
