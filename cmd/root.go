// Code generated by cli-generator; DO NOT EDIT.
package cmd



import (
	
	"github.com/spf13/viper"
	"fmt"
	"os"
	
	"github.com/spf13/cobra"
)
var cnf *viper.Viper = viper.New()

var rootCmdOptions struct {
	
}

var rootCmd = &cobra.Command{
	Use:     "trello-sync",
	Version: "0.1.0",
	Long: "Sync Trello board to Google Speadsheet",
	PreRun: func(cmd *cobra.Command, args []string) {
		
	},
}



// Execute - execute the root command
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}


func init() {
}