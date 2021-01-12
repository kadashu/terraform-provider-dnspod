module github.com/cofyc/terraform-provider-dnspod

go 1.15

require (
	github.com/hashicorp/terraform-plugin-sdk v1.16.0
	github.com/nrdcg/dnspod-go v0.4.0
)

replace github.com/nrdcg/dnspod-go => github.com/cofyc/dnspod-go v0.4.1-0.20210112103752-82f388487682
