# envsubst

This is a fork of <https://github.com/dnitsch/envsubst> where the `NoReplace` restriction has been added.

This fork [improves](https://github.com/hellt/envsubst/pull/1) the NoReplace restriction by not replacing env variables defined with the default like this:

For this input:

```
ExistingEnvVarIsReplaced: $REPLACE
NoReplaceNotToBeUsedWithDefault: ${someVarWithDefault:=myDefault}
NoReplaceShouldNotReplaceNonExistingEnvVar: $ToIgnore
```

With these env vars set:

```
REPLACE=bar
```

Expected output is:

```
ExistingEnvVarIsReplaced: bar
NoReplaceNotToBeUsedWithDefault: myDefault
NoReplaceShouldNotReplaceNonExistingEnvVar: $ToIgnore
```
