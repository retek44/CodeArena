# Migration lint policy (atlas migrate lint, Community Edition; the PR workflow runs it
# on the migrations a pull request adds). Expand/contract: every migration must keep
# the previous release working, so destructive, data-dependent and backward-incompatible
# changes fail the build. Split such a change into expand, deploy, contract instead.
lint {
  destructive {
    error = true
  }
  data_depend {
    error = true
  }
  incompatible {
    error = true
  }
}
