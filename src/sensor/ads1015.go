package battery

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ADS1015 represents the ADS1015 ADC object.
type ADS1015 struct {
	ADCPath           string
	RawVoltageFiles   []string
	VoltageScaleFiles []string
	VoltageScales     []string
	SampleFreqFiles   []string
	SettingsDir       string

	sampleFreq string // sample frequency in Hz
}

// NewADS1015 creates a new instance of ADS1015 with default values.
func NewADS1015() *ADS1015 {
	return &ADS1015{
		ADCPath:           "/sys/bus/i2c/devices/5-0049/iio:device1",
		RawVoltageFiles:   []string{"in_voltage0_raw", "in_voltage1_raw", "in_voltage2_raw", "in_voltage3_raw"},
		VoltageScaleFiles: []string{"in_voltage0_scale", "in_voltage1_scale", "in_voltage2_scale", "in_voltage3_scale"},
		VoltageScales:     []string{"2\n", "2\n", "1\n", "2\n"},
		SampleFreqFiles:   []string{"in_voltage0_sampling_frequency", "in_voltage1_sampling_frequency", "in_voltage2_sampling_frequency", "in_voltage3_sampling_frequency"},
		SettingsDir:       "/sys/bus/i2c/devices/i2c-5/",
		sampleFreq:        "1600\n",
	}
}

// InitSettings initializes the ADS1015 device settings.
func (a *ADS1015) InitSettings() error {
	err := a.writeToFile(filepath.Join(a.SettingsDir, "new_device"), "ads1015 0x49\n")
	if err != nil {
		log.Err(err).Msg("Failed to initialize ADS1015 device")
		return err
	}

	// Set scales for all channels
	for idx, voltageScaleFile := range a.VoltageScaleFiles {
		err := a.writeToFile(filepath.Join(a.ADCPath, voltageScaleFile), a.VoltageScales[idx])
		if err != nil {
			log.Err(err).Msgf("Failed to set scale for %s", voltageScaleFile)
			return err
		}
	}

	// Set the sample frequency for each channel
	for _, SampleFreqFiles := range a.SampleFreqFiles {
		err := a.writeToFile(filepath.Join(a.ADCPath, SampleFreqFiles), a.sampleFreq)
		if err != nil {
			log.Err(err).Msgf("Failed to set sample frequency for %s", SampleFreqFiles)
			return err
		}
	}

	// Wait for setup
	time.Sleep(1 * time.Second)
	return nil
}

// GetSettings prints the current settings of the ADS1015 device.
func (a *ADS1015) GetSettings() error {
	files := append(a.VoltageScaleFiles, a.SampleFreqFiles...)
	files = append(files, a.RawVoltageFiles...)
	for _, file := range files {
		data, err := a.readFromFile(filepath.Join(a.ADCPath, file))
		if err != nil {
			log.Err(err).Msgf("Failed to read %s", file)
			return err
		}
		fmt.Printf("%s: %s\n", file, data)
	}
	return nil
}

func (a *ADS1015) GetPercentage(idx int) (float64, error) {
	voltage, err := a.GetVoltage(idx)
	if err != nil {
		log.Err(err).Msg("Failed to get voltage")
		return 0, err
	}
	return 4096 / 3300 * voltage / 2048 * 100, nil
}

// GetVoltage retrieves the voltage at a specific index.
func (a *ADS1015) GetVoltage(idx int) (float64, error) {
	if idx < 0 || idx >= len(a.RawVoltageFiles) {
		return 0, fmt.Errorf("index out of range")
	}

	rawData, err := a.readFromFile(filepath.Join(a.ADCPath, a.RawVoltageFiles[idx]))
	if err != nil {
		log.Err(err).Msgf("Failed to read %s", a.RawVoltageFiles[idx])
		return 0, err
	}
	scaleData, err := a.readFromFile(filepath.Join(a.ADCPath, a.VoltageScaleFiles[idx]))
	if err != nil {
		log.Err(err).Msgf("Failed to read %s", a.VoltageScaleFiles[idx])
		return 0, err
	}

	rawValue, err := strconv.ParseFloat(strings.TrimSpace(rawData), 64)
	if err != nil {
		log.Err(err).Msg("Failed to convert raw data to integer")
		return 0, err
	}
	scaleValue, err := strconv.ParseFloat(strings.TrimSpace(scaleData), 64)
	if err != nil {
		log.Err(err).Msg("Failed to convert scale data to integer")
		return 0, err
	}

	// Calculate the voltage based on the raw and scale values.
	voltage := rawValue * (2.048 / 2048.0) * scaleValue

	// ADC scale=1 and x10 for Vbat. Driver does not support x10 scale.
	if idx == 2 {
		voltage *= 10
	}

	return voltage, nil
}

// GetVoltages prints the current voltages of the ADS1015 device.
func (a *ADS1015) GetAllVoltages() error {
	for i := 0; i < len(a.RawVoltageFiles); i++ {
		v, err := a.GetVoltage(i)
		if err != nil {
			log.Err(err).Msgf("Failed to get voltage at index %d", i)
			return err
		}
		log.Debug().Float64("voltage", v).Msgf("Voltage at index %d", i)
	}
	return nil
}

// Delete removes the ADS1015 device.
func (a *ADS1015) Delete() error {
	err := a.writeToFile(filepath.Join(a.SettingsDir, "delete_device"), "0x49\n")
	if err != nil {
		log.Err(err).Msg("Failed to delete ADS1015 device")
	}
	return err
}

func (a *ADS1015) writeToFile(filePath, content string) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0644)
	if err != nil {
		log.Err(err).Msg("Failed to open file")
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		log.Err(err).Msg("Failed to write to file")
		return err
	}
	// Ensure data is written to the file before returning.
	return file.Sync()
}

func (a *ADS1015) readFromFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Err(err).Msg("Failed to open file")
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	return scanner.Text(), scanner.Err()
}
