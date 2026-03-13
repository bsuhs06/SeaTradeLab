#!/usr/bin/env python3
"""
Run from the analytics/ directory:
    python -m src.run              # summary report
    python -m src.run --detect     # run all detectors and persist results
    python -m src.run --sts        # run STS detection only
    python -m src.run --gaps       # run AIS gap detection only
"""

import sys
import argparse
from datetime import datetime
from .database import russian_vessels
from .analyzers import (
    find_proximity_events,
    flag_russian_sts,
    persist_events,
    detect_port_visits,
    find_dark_vessels,
    detect_ais_gaps,
    persist_gaps,
    detect_russian_port_visits,
    persist_port_visits,
)


def run_summary():
    """Print a summary of the current data."""
    rv = russian_vessels()
    print(f"Russian vessels in database: {len(rv)}")
    if not rv.empty and "name" in rv.columns:
        named = rv[rv["name"].notna()]
        print(f"  With names: {len(named)}")
    print()


def run_sts_detection(hours=24, distance_m=500, speed_kts=3.0, min_duration=15, persist=True):
    """Run STS proximity detection and optionally persist results."""
    print(f"Scanning last {hours}h for STS events (dist={distance_m}m, speed<{speed_kts}kn, min_dur={min_duration}min)...")
    events = find_proximity_events(
        hours=hours,
        distance_threshold_m=distance_m,
        speed_threshold_kts=speed_kts,
        min_duration_minutes=min_duration,
    )
    if events.empty:
        print("  No STS events detected.")
        return events

    print(f"  Found {len(events)} potential STS events")

    russian = events[
        events["mmsi_a"].astype(str).str.startswith("273")
        | events["mmsi_b"].astype(str).str.startswith("273")
    ]
    if not russian.empty:
        print(f"  {len(russian)} involve Russian-flagged vessels")

    if persist:
        count = persist_events(events)
        print(f"  Persisted {count} new events to database")

    print(events.to_string(index=False))
    return events


def run_gap_detection(hours_back=48, min_gap_hours=2, persist=True):
    """Run AIS gap detection and optionally persist results."""
    print(f"Scanning last {hours_back}h for AIS gaps >= {min_gap_hours}h...")
    gaps = detect_ais_gaps(hours_back=hours_back, min_gap_hours=min_gap_hours)
    if gaps.empty:
        print("  No AIS gaps detected.")
        return gaps

    print(f"  Found {len(gaps)} AIS gaps")

    if persist:
        count = persist_gaps(gaps)
        print(f"  Persisted {count} gap records to database")

    # Also show currently dark vessels
    dark = find_dark_vessels(min_gap_hours=6)
    if not dark.empty:
        print(f"  Currently dark vessels (6h+): {len(dark)}")

    return gaps


def run_port_tracking(hours=48):
    """Run port visit detection."""
    print(f"Detecting port visits (last {hours}h, Russian vessels)...")
    visits = detect_port_visits(hours=hours)
    if visits.empty:
        print("  No port visits detected.")
    else:
        print(f"  Found {len(visits)} port visits")
        print(visits.to_string(index=False))
    return visits


def run_russian_port_visits(hours=168, min_duration=30, persist=True):
    """Detect all vessels visiting Russian ports."""
    print(f"Scanning for vessels visiting Russian ports (last {hours}h, min {min_duration}min)...")
    visits = detect_russian_port_visits(hours=hours, min_duration_minutes=min_duration)
    if visits.empty:
        print("  No Russian port visits detected.")
        return visits

    non_russian = visits[~visits["is_russian"]]
    if not non_russian.empty:
        print(f"\n  Non-Russian vessels in Russian ports:")
        for _, v in non_russian.iterrows():
            status = " [STILL IN PORT]" if v["still_in_port"] else ""
            print(f"    {v['flag_country']:20s} | {v['vessel_name'] or v['mmsi']:25s} | {v['vessel_type'] or 'Unknown':20s} | {v['port_name']:15s} | {v['duration_hours']:.1f}h{status}")

    if persist:
        count = persist_port_visits(visits)
        print(f"\n  Persisted {count} port visit records")

    return visits


def main():
    parser = argparse.ArgumentParser(
        description="SeaTradeLab analytics"
    )
    parser.add_argument("--detect", action="store_true", help="Run all detectors and persist")
    parser.add_argument("--sts", action="store_true", help="Run STS detection only")
    parser.add_argument("--gaps", action="store_true", help="Run AIS gap detection only")
    parser.add_argument("--ports", action="store_true", help="Run port tracking only")
    parser.add_argument("--russian-ports", action="store_true", help="Detect vessels visiting Russian ports")
    parser.add_argument("--hours", type=int, default=24, help="Hours to look back (default: 24)")
    parser.add_argument("--distance", type=float, default=500, help="STS distance threshold in meters (default: 500)")
    parser.add_argument("--speed", type=float, default=3.0, help="STS speed threshold in knots (default: 3.0)")
    parser.add_argument("--min-duration", type=int, default=15, help="STS min duration in minutes (default: 15)")
    parser.add_argument("--gap-hours", type=float, default=2.0, help="Min AIS gap hours (default: 2.0)")
    parser.add_argument("--no-persist", action="store_true", help="Don't write results to database")
    args = parser.parse_args()

    print(f"=== SeaTradeLab Analytics - {datetime.now().strftime('%Y-%m-%d %H:%M')} ===")

    persist = not args.no_persist

    if args.detect or (not args.sts and not args.gaps and not args.ports and not args.russian_ports):
        # Run everything
        run_summary()
        print()
        run_sts_detection(
            hours=args.hours,
            distance_m=args.distance,
            speed_kts=args.speed,
            min_duration=args.min_duration,
            persist=persist,
        )
        print()
        run_gap_detection(
            hours_back=args.hours * 2,
            min_gap_hours=args.gap_hours,
            persist=persist,
        )
        print()
        run_port_tracking(hours=48)
        print()
        run_russian_port_visits(hours=args.hours * 7, persist=persist)
    else:
        if args.sts:
            run_sts_detection(
                hours=args.hours,
                distance_m=args.distance,
                speed_kts=args.speed,
                min_duration=args.min_duration,
                persist=persist,
            )
        if args.gaps:
            run_gap_detection(
                hours_back=args.hours * 2,
                min_gap_hours=args.gap_hours,
                persist=persist,
            )
        if args.ports:
            run_port_tracking(hours=args.hours)
        if args.russian_ports:
            run_russian_port_visits(hours=args.hours * 7, persist=persist)

    print("\nDone.")


if __name__ == "__main__":
    main()
