provider "nomad" {}
variable image_repository {
  default = "vxlabs/wasp"
}
variable image_tag {
    default = "latest"
}

resource "nomad_job" "messages" {
  jobspec = templatefile("${path.module}/template.nomad.hcl",
    {
      service_image        = var.image_repository,
      service_version        = var.image_tag,
    },
  )
}
