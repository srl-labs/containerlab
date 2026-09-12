# Developers Guide

Containerlab relies on contributions from the community to improve existing functionality and add new features. The developers guide provides information on how to contribute to various bits of the project.

Remember, that all contributions are welcome and important, no matter how big or small they are. Spot a typo in the documentation? Found a bug? Have an idea for a new feature? Want to improve the performance of the code? All of these are valuable contributions to the project.

Thank you for considering contributing to containerlab!

## Git LFS

Containerlab repo stores raster, video and other graphics using [Git Large File Storage (LFS)](https://git-lfs.com/). Contributors should have Git LFS installed in their system and initialize it in their clone:

```bash
git lfs install
```

When a maintainer pushes to a contributor's pull request branch, GitHub may reject the LFS lock check with `You must have push access to verify locks`, even when the contributor allows maintainer edits. Containerlab does not use LFS file locking, so disable this check locally:

```bash
git config --local 'lfs.https://github.com/.locksverify' false
```
