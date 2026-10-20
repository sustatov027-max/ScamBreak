package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sustatov027-max/ScamBreak/pkg/auth"
)

func main() {
	privPath := flag.String("priv", "certs/jwt_private.pem", "Path to save private RSA key")
	pubPath := flag.String("pub", "certs/jwt_public.pem", "Path to save public RSA key")
	bits := flag.Int("bits", 2048, "RSA key size in bits")
	flag.Parse()

	fmt.Printf("Generating %d-bit RSA key pair...\n", *bits)
	if err := auth.SaveRSAKeyPairToFiles(*privPath, *pubPath, *bits); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating RSA keys: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("RSA key pair generated successfully:")
	fmt.Printf("  - Private key: %s\n", *privPath)
	fmt.Printf("  - Public key:  %s\n", *pubPath)
}
