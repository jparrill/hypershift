package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	paygencmd "github.com/openshift/hypershift/ignition-server/payload-generator-server/cmd"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.JSONEncoder(func(o *zapcore.EncoderConfig) {
		o.EncodeTime = zapcore.RFC3339TimeEncoder
	})))

	root := &cobra.Command{
		Use:   "debug",
		Short: "Flag to debug the ignition server payload generator API",
	}

	root.AddCommand(paygencmd.NewRunHTTPIgnitionPayloadGenerator())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
