# Changelog

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
