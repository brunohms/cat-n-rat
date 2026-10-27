locals {
  repo_root = abspath("${path.module}/../..")

  # Each image is tagged with a hash of its sources, so a code change produces a new tag
  # and Helm rolls the pods; an unchanged tree keeps the same tag and nothing is rebuilt.
  images = {
    backend = {
      repository = "catnrat-backend"
      context    = "${local.repo_root}/app/backend"
      files      = setsubtract(fileset("${local.repo_root}/app/backend", "**"), fileset("${local.repo_root}/app/backend", "bin/**"))
    }
    frontend = {
      repository = "catnrat-frontend"
      context    = "${local.repo_root}/app/frontend"
      files = setunion(
        fileset("${local.repo_root}/app/frontend", "src/**"),
        ["Dockerfile", "index.html", "nginx.conf.template", "package.json", "package-lock.json", "tsconfig.json", "vite.config.ts"],
      )
    }
    generator = {
      repository = "purchase-generator"
      context    = "${local.repo_root}/generator"
      files      = setunion(fileset("${local.repo_root}/generator", "app/**/*.py"), ["Dockerfile", "requirements.txt"])
    }
  }

  image_tags = {
    for name, image in local.images : name => substr(sha256(join("", [
      for file in sort(tolist(image.files)) : filesha256("${image.context}/${file}")
    ])), 0, 12)
  }
}

# Builds the image with Docker and loads it into the kind nodes (requires the docker and kind CLIs)
resource "terraform_data" "image" {
  for_each = local.images

  triggers_replace = {
    cluster = kind_cluster.this.id
    tag     = local.image_tags[each.key]
  }

  provisioner "local-exec" {
    command = <<-EOT
      set -eu
      docker build -t "$IMAGE" "$CONTEXT"
      kind load docker-image "$IMAGE" --name "$CLUSTER"
    EOT
    environment = {
      IMAGE   = "${each.value.repository}:${local.image_tags[each.key]}"
      CONTEXT = each.value.context
      CLUSTER = kind_cluster.this.name
    }
  }
}
