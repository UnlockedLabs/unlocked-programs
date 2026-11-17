# Changelog

## [0.2.0](https://github.com/UnlockedLabs/unlocked-programs/compare/v0.1.3...v0.2.0) (2026-11-13)


### Features

* list the facilities a program is offered at on the programs overview (EN-119) ([f77aa50](https://github.com/UnlockedLabs/unlocked-programs/commit/f77aa502235abef6dcc0882ed97693066c255f17))
* list the facilities a program is offered at on the programs overview (EN-119) ([f77aa50](https://github.com/UnlockedLabs/unlocked-programs/commit/f77aa502235abef6dcc0882ed97693066c255f17))
* list the facilities a program is offered at on the programs overview (EN-119) ([cd19b12](https://github.com/UnlockedLabs/unlocked-programs/commit/cd19b120a15db098d1f813e252ff21f4f2a4c423))


### Bug Fixes

* spell Transferred correctly everywhere (EN-244) ([#6](https://github.com/UnlockedLabs/unlocked-programs/issues/6)) ([57ff1a7](https://github.com/UnlockedLabs/unlocked-programs/commit/57ff1a7efec6b771deee3815e444342fa6c740ce))
* stop counting a program at facilities it was removed from (EN-246) ([c275850](https://github.com/UnlockedLabs/unlocked-programs/commit/c275850a4ca1786252b560cdcad08ffb69e1c864))
* stop residents being eligible for a program's classes after it leaves their facility (EN-246) ([5fb0c6e](https://github.com/UnlockedLabs/unlocked-programs/commit/5fb0c6e491cc615c816ab2863053789818017a21))
* stop residents being eligible for a program's classes after it leaves their facility (EN-246) ([5fb0c6e](https://github.com/UnlockedLabs/unlocked-programs/commit/5fb0c6e491cc615c816ab2863053789818017a21))

## [0.1.3](https://github.com/UnlockedLabs/unlocked-programs/compare/v0.1.2...v0.1.3) (2026-11-11)


### Bug Fixes

* answer 400 for invalid program status bodies (EN-243) ([#10](https://github.com/UnlockedLabs/unlocked-programs/issues/10)) ([9b31a3f](https://github.com/UnlockedLabs/unlocked-programs/commit/9b31a3f13035405c07496e995f133a33c78df409))

## [0.1.2](https://github.com/UnlockedLabs/unlocked-programs/compare/v0.1.1...v0.1.2) (2026-11-11)


### Bug Fixes

* build with Go 1.26.11 for GO-2026-6859 (EN-251) ([57895db](https://github.com/UnlockedLabs/unlocked-programs/commit/57895db59f9da8b5d3262c94e04d3ca764e71408))
* build with Go 1.26.11 for GO-2026-6859 (EN-251) and govulncheck consistency update- [#5](https://github.com/UnlockedLabs/unlocked-programs/issues/5) ([84fba8f](https://github.com/UnlockedLabs/unlocked-programs/commit/84fba8f02d9d7df34364f253bdc7a21d0f85e47e))
* build with Go 1.26.11 for GO-2026-6859 (EN-251) and govulncheck consistency update- [#5](https://github.com/UnlockedLabs/unlocked-programs/issues/5) ([84fba8f](https://github.com/UnlockedLabs/unlocked-programs/commit/84fba8f02d9d7df34364f253bdc7a21d0f85e47e))

## [0.1.1](https://github.com/UnlockedLabs/unlocked-programs/compare/v0.1.0...v0.1.1) (2026-11-10)


### Bug Fixes

* allowlist order_by and direction on list queries (EN-229) ([#5](https://github.com/UnlockedLabs/unlocked-programs/issues/5)) ([bf809f1](https://github.com/UnlockedLabs/unlocked-programs/commit/bf809f1447e5851ed74b59281930f15051c8dc36))
* check the class's facility before showing its enrollments or conflicts (EN-238) ([afe6029](https://github.com/UnlockedLabs/unlocked-programs/commit/afe602910164d0fd2202d3079e0208cd8c05ce62))
* only allow bulk class edits when every class is at the admin's facility (EN-239) ([23f5f43](https://github.com/UnlockedLabs/unlocked-programs/commit/23f5f43ad1d801d3d146e1577e4cf8cfba95eeb0))
* only allow bulk class edits when every class is at the admin's facility (EN-239) ([d5aff3e](https://github.com/UnlockedLabs/unlocked-programs/commit/d5aff3e7a25e6a1b4868b76163bf89de93ab0568))
* parse bulk class IDs once for the access check and the handler (EN-239) ([0c4ffae](https://github.com/UnlockedLabs/unlocked-programs/commit/0c4ffaeb0ad1f94e26aac66382fbaca9503dddd0))
* remove the class enrollments delete route that deleted the wrong enrollment (EN-242) ([483c79c](https://github.com/UnlockedLabs/unlocked-programs/commit/483c79c371a9ca78441ca06c197e8fb50c1ae837))
* remove the class enrollments delete route that deleted the wrong enrollment (EN-242) ([f6dfdb2](https://github.com/UnlockedLabs/unlocked-programs/commit/f6dfdb29cf22b6d1b582616903096c4cb41e6af4))

## 0.1.0 (2026-11-06)


### Features

* add class activation to the worker, moved from core (EN-207) ([e255e14](https://github.com/UnlockedLabs/unlocked-programs/commit/e255e140fa28514bdffad2a66980815add49cd8c))
* add Programs' database queries and migrations, moved from core (EN-207) ([1b48de7](https://github.com/UnlockedLabs/unlocked-programs/commit/1b48de71208c1819f6b2ad55349accd516be509b))
* add Programs' models, moved from core (EN-207) ([ccf50f7](https://github.com/UnlockedLabs/unlocked-programs/commit/ccf50f7ff0cff4bc3bcb5862f8463665f83a2ff3))
* add the Programs routes and the answers core asks for (EN-207) ([6395625](https://github.com/UnlockedLabs/unlocked-programs/commit/63956253c798dc454e221ae557bc26c56d70aa83))
* add the sample data core's make seed asks for (EN-207) ([1877828](https://github.com/UnlockedLabs/unlocked-programs/commit/1877828994862ab9c04d8d9fbf58c1b29a02b5be))
* add the worker, migrate and migrate-reset commands (EN-207) ([4a44273](https://github.com/UnlockedLabs/unlocked-programs/commit/4a442736a5ea8940838af444c77da09a2c04ac2e))
* move change_log_entries into the programs schema (EN-207) ([e746eae](https://github.com/UnlockedLabs/unlocked-programs/commit/e746eae86ab9cdd7f1f55b7cffa923618a438090))
* run Programs as its own service (EN-207) ([1e177e7](https://github.com/UnlockedLabs/unlocked-programs/commit/1e177e7fc2d14c683f8c0e760d0f221d55b8f8a3))
* run Programs as its own service (EN-207) ([1e177e7](https://github.com/UnlockedLabs/unlocked-programs/commit/1e177e7fc2d14c683f8c0e760d0f221d55b8f8a3))
* serve the Programs API and render its PDF reports (EN-207) ([3d2df37](https://github.com/UnlockedLabs/unlocked-programs/commit/3d2df37a747b2b16f9d9c0c0ceca459ae2b62038))


### Bug Fixes

* build and check with Go 1.26.10 (EN-207) ([786cf22](https://github.com/UnlockedLabs/unlocked-programs/commit/786cf22187e754fb27e7d0609cd59cfc729557dd))
* write transfers with the same enrollment status the app reads (EN-207) ([3e4bb6a](https://github.com/UnlockedLabs/unlocked-programs/commit/3e4bb6ac5fb041f36c7d07a5ffcebdde727e9692))
