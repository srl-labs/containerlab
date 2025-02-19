# Versions

The `latest` version file contains the latest public version of the containerlab project. It is used in the `get.sh` script to detect the latest available GA version.

When a new containerlab release is published, the developers should invoke:

```bash
make tag-release VERSION=x.y.z
```

where `vx.y.z` is the new version number. This will

* put the new release version in the `./internal/versions/latest` file.
* add all files in the staging aread and record a commit
* add a tag to the new commit

> The version should be passed without the `v` prefix.
