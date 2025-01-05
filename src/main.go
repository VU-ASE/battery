package main

import (
	"fmt"
	"os"
	"os/exec"

	// "os/signal"
	// "syscall"
	"time"

	battery "vu/ase/battery/src/sensor"

	pb_outputs "github.com/VU-ASE/rovercom/packages/go/outputs"
	roverlib "github.com/VU-ASE/roverlib-go/src"
	"github.com/rs/zerolog/log"
)

const voltageLine = 2

const sampleRate = 5
const shutdownSys = true
const batteryKill = 14.9
const batteryWarn = 15.4

var voltageValue float64 = 0

var bat *battery.ADS1015

func exit_safely(bat *battery.ADS1015) {
	err := bat.Delete()
	if err != nil {
		log.Err(err).Msg("Failed to delete bat")
	}
}

// Battery data
func critical_loop() error {
	// Initialize the battery sensor
	bat = battery.NewADS1015()
	defer exit_safely(bat) // Ensure cleanup

	// nolint: errcheck
	_ = bat.Delete() // Delete the battery block dev if it already exists
	err := bat.InitSettings()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize settings")
		return err
	}

	warningDisplayed := false
	for {

		time.Sleep(time.Duration(sampleRate) * time.Second)
		v, err := bat.GetVoltage(voltageLine)
		voltageValue = v
		if err != nil {
			log.Err(err).Msg("Failed to get battery voltage")
			continue
		}

		log.Debug().Float64("voltage", voltageValue).Msg("Battery voltage")

		if shutdownSys && voltageValue <= float64(batteryKill) {
			log.Warn().Msg("Battery voltage is critically low. Shutting down the system")
			err := exec.Command("sudo", "shutdown", "now").Run()
			if err != nil {
				log.Err(err).Msg("Failed to execute shutdown command")
			}
		} else if voltageValue <= float64(batteryWarn) {
			log.Warn().Msg("Battery voltage is low")
			if !warningDisplayed { /* Only display warning once! */
				err := exec.Command("wall", fmt.Sprintf("Battery voltage is low: %f", voltageValue)).Run()
				if err != nil {
					log.Err(err).Msg("Failed to execute 'wall' command")
				}
				warningDisplayed = true
			}
		}
	}
}

// Trivial publisher which reads battery voltage and publishes it
func publisher(output roverlib.WriteStream) {
	for {
		time.Sleep(time.Duration(sampleRate) * time.Second)

		msg := pb_outputs.SensorOutput{
			SensorId:  5,
			Timestamp: uint64(time.Now().UnixMilli()),
			Status:    0,
			SensorOutput: &pb_outputs.SensorOutput_BatteryOutput{
				BatteryOutput: &pb_outputs.BatterySensorOutput{
					CurrentOutputVoltage: float32(voltageValue),
					WarnVoltage:          float32(batteryWarn),
					KillVoltage:          float32(batteryKill),
				},
			},
		}

		err := output.Write(&msg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send protobuf message")
			continue
		}
	}
}

func run(
	service roverlib.Service,
	config *roverlib.ServiceConfiguration,
) error {
	output := service.GetWriteStream("voltage")

	// Publish battery data in an auto restarting fashion
	go func() {
		for {
			/* Auto restart on error */
			publisher(*output)
			time.Sleep(5 * time.Second) // Avoid spamming logs...
		}
	}()

	select {} /* Block forever */
}

func onTerminate(sig os.Signal) error {
	time.Sleep(50 * time.Millisecond)
	log.Warn().Str("signal", sig.String()).Msg("Killed battery sensor, exiting safely")
	exit_safely(bat)
	os.Exit(0)
	return nil
}

func main() {
	/* Start critical loop in a self restarting way */
	go func() {
		for {
			var err = critical_loop()
			if err != nil {
				return
			}
			/* Sleep for a while before restarting */
			time.Sleep(5 * time.Second)
		}
	}()

	roverlib.Run(run, onTerminate)
}
