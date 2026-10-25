#!/bin/bash

# first extract chassis info file from srsim then use the script to update the schema enums.

CHASSIS_INFO_FILE=chassis_info.json
SCHEMA=../schemas/clab.schema.json

jq --indent 4 -s '
  .[0] as $all |
  .[1] |
  ([.. | objects | select(.enum? and (.enum | index("ixr-e2n"))) | .enum][0]) as $node_types |
  ($all | with_entries(select((.key | ascii_downcase) as $key | $node_types | index($key)))) as $c |
  ($c | [.[].cpm // [] | .[]] + [.[].fixed_hw_cfg // empty]) as $cpms |
  ($c | [.[].iom // [] | .[]] + [.[].fixed_hw_cfg // empty]) as $cards |
  ($c | [.[].sfm // [] | .[]]) as $sfms |
  ($c | [.[].mda // [] | .[] | select(. != null)] + [.[].fixed_mdas // {} | .[]]) as $mdas |
  ($c | [.[].xiom // [] | .[]]) as $xioms |

  .definitions["sros-cpm-types"].enum |= (. + $cpms | unique | sort) |
  .definitions["sros-card-types"].enum |= (. + $cards | unique | sort) |
  .definitions["sros-sfm-types"].enum |= (. + $sfms | unique | sort) |
  .definitions["sros-mda-types"].enum |= (. + $mdas | unique | sort) |
  .definitions["sros-xiom-types"].enum |= (. + $xioms | unique | sort)
' "$CHASSIS_INFO_FILE" "$SCHEMA" > "$SCHEMA.tmp"

mv "$SCHEMA.tmp" "$SCHEMA"