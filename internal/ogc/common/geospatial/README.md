# Geospatial data resources / collections.

For Gomagpie devs: If you want to implement collections support in one of the OGC building blocks 
in Gomagpie (see `ogc` package) you'll need to perform the following tasks:

Config:
- Expand / add yaml tag in `engine.Config.OgcAPI` to allow users to configure collections

OpenAPI
- Materialize the collections as API endpoints by looping over the collection in the OpenAPI template 
  for that specific OGC building block. For example for OGC tiles you'll need to 
  create `/collections/{collectionId}/tiles` endpoints in OpenAPI. Note `/collections/{collectionId}` endpoint
  are already implemented in OpenAPI by this package.

Responses:
- Expand the `collections` and `collection` [templates](./templates). 
- Implement an endpoint in your specific OGC API building block to serve the CONTENTS of a collection 
  (e.g. `/collections/{collectionId}/tiles`)

Testing:
- Add unit tests

