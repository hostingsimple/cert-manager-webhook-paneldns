package main

import (
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/Veeau/cert-manager-webhook-paneldns/solver"
)

// GroupName is the ACME DNS-01 webhook group name.
// Must match the groupName in your ClusterIssuer config.
const GroupName = "acme.paneldns.com"

func main() {
	cmd.RunWebhookServer(GroupName, &solver.PanelDNSSolver{})
}
