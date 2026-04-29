import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Overview

:::warning[Important]

`battery` is known as a daemon service. You do not need to manage this service yourself. For more information, please visit the page about [`roverd`'s daemon services](https://ase.vu.nl/docs/tutorials/knowing-more/roverd-daemons).

:::

## Purpose 

The `battery` service uses the ADS1015 voltage sensor to track the battery charging level. If the voltage drops below a [warning threshold](https://github.com/VU-ASE/battery/blob/0929a05129b1a065dcd423aa189d056eaeec73a6/src/main.go#L24) it will send a warning to active SSH users. If the voltage drops below a [critical threshold](https://github.com/VU-ASE/battery/blob/0929a05129b1a065dcd423aa189d056eaeec73a6/src/main.go#L23) it will shut off the Debix to prevent undercharging in order to improve battery aging.

**NB**: the warning and shutdown thresholds are based on the battery properties (cell count and nominal voltage.)


## Requirements

- The ADS1015 voltage sensor should be connected to the Rover

## Inputs

As defined in the [*service.yaml*](https://github.com/VU-ASE/battery/blob/main/service.yaml), this service does not depend on any other services.

## Outputs

As defined in the [*service.yaml*](https://github.com/VU-ASE/battery/blob/main/service.yaml), this service does not expose any write streams.