package main

import (
	"fmt"
	"os"
	"os/exec"
	// "os/signal"
	// "syscall"
	"time"

	battery "vu/ase/battery/src/sensor"

	pb_outputs "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/outputs"
	pb_systemManager_messages "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/systemmanager"
	serviceRunner "github.com/VU-ASE/pkg-ServiceRunner/v2/src"
	zmq "github.com/pebbe/zmq4"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

const voltageLine = 2

const sampleRate = 5
const shutdownSys = true
const batteryKill = 14.9
const batteryWarn = 15.4

var voltageValue float64 = 0

var bat *battery.ADS1015;

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

	return nil
}

// Trivial publisher which reads battery voltage and publishes it
func publisher(outputAddr string) {
	publisher, _ := zmq.NewSocket(zmq.PUB)
	defer publisher.Close()

	err := publisher.Bind(outputAddr)
	if err != nil {
		log.Err(err).Msg("Failed to bind publisher")
		return
	}

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

		dataBytes, err := proto.Marshal(&msg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal protobuf message")
			continue
		}

		_, err = publisher.SendBytes(dataBytes, 0)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send protobuf message")
			continue
		}
	}
}

func run(
	serviceInfo serviceRunner.ResolvedService,
	sysMan serviceRunner.SystemManagerInfo,
	_ *pb_systemManager_messages.TuningState) error {

	outputAddr, err := serviceInfo.GetOutputAddress("battery-voltage")
	if err != nil {
		log.Err(err).Msg("Failed to get tuning values")
		return err
	}

	// Publish battery data in an auto restarting fashion
	go func() {
		for {
			/* Auto restart on error */
			publisher(outputAddr)
			time.Sleep(5 * time.Second) // Avoid spamming logs...
		}
	}()

	select {} /* Block forever */
}

func tuningCallback(_ *pb_systemManager_messages.TuningState) {
	log.Info().Msg("Tuning state changed - ignored.")
}


func onTerminate(sig os.Signal) {
	time.Sleep(50 * time.Millisecond)
	log.Warn().Str("signal", sig.String()).Msg("mod-BatterySensor onTerminate, exiting safely")
	exit_safely(bat)
	os.Exit(0)
}

func main() {
	/* Start critical loop in a self restarting way */
	go func() {
		for {
			critical_loop()
			/* Sleep for a while before restarting */
			time.Sleep(5 * time.Second)
		}
	}()

	serviceRunner.Run(run, tuningCallback, onTerminate, false)
}
