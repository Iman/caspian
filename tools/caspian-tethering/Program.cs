// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// caspian-tethering: the one piece of the Windows appliance that is not Go.
//
// Windows exposes Mobile Hotspot only through WinRT
// (Windows.Networking.NetworkOperators.NetworkOperatorTetheringManager), and
// Go has no projection of that namespace. This program is the smallest thing
// that can call it: it reads ONE JSON request on standard input, performs ONE
// action, prints ONE JSON line on standard output and exits. It keeps no
// state, opens no listener and takes no arguments beyond the operation name,
// which is repeated on the command line so that a process listing shows what
// it is doing without showing the passphrase.
//
// The contract is internal/hotspot/mobilehotspot.go's TetheringRequest and
// TetheringReply; the field names here are theirs.
//
// Every failure is reported as a reply with ok=false, the Windows enum name in
// "code" and a sentence in "error". Nothing here throws to the console: an
// unhandled exception would print a stack trace with no code for the caller
// to map onto a fault the user can act on.

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.NetworkInformation;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Threading.Tasks;
using Windows.Networking.Connectivity;
using Windows.Networking.NetworkOperators;

namespace Caspian.Tethering
{
    internal sealed class Request
    {
        [JsonPropertyName("op")] public string Op { get; set; } = "";
        [JsonPropertyName("uplink")] public string? Uplink { get; set; }
        [JsonPropertyName("adapter")] public string? Adapter { get; set; }
        [JsonPropertyName("ssid")] public string? Ssid { get; set; }
        [JsonPropertyName("passphrase")] public string? Passphrase { get; set; }
        [JsonPropertyName("band")] public string? Band { get; set; }
    }

    internal sealed class Client
    {
        [JsonPropertyName("mac")] public string Mac { get; set; } = "";
        [JsonPropertyName("hostnames")] public List<string>? Hostnames { get; set; }
    }

    internal sealed class Reply
    {
        [JsonPropertyName("ok")] public bool Ok { get; set; }
        [JsonPropertyName("state")] public string State { get; set; } = "unknown";
        [JsonPropertyName("ssid")] public string? Ssid { get; set; }
        [JsonPropertyName("band")] public string? Band { get; set; }
        [JsonPropertyName("clients")] public List<Client>? Clients { get; set; }
        [JsonPropertyName("error")] public string? Error { get; set; }
        [JsonPropertyName("code")] public string? Code { get; set; }
    }

    internal static class Program
    {
        private static readonly JsonSerializerOptions JsonOptions = new()
        {
            DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        };

        private static async Task<int> Main(string[] args)
        {
            Reply reply;
            try
            {
                var text = await Console.In.ReadToEndAsync();
                var req = JsonSerializer.Deserialize<Request>(text, JsonOptions);
                if (req is null || string.IsNullOrEmpty(req.Op))
                {
                    reply = Fail("BadRequest", "no request was read on standard input");
                }
                else if (args.Length > 0 && args[0] != req.Op)
                {
                    reply = Fail("BadRequest", "the operation on the command line and in the request differ");
                }
                else
                {
                    reply = await Run(req);
                }
            }
            catch (Exception e)
            {
                reply = Fail(e.GetType().Name, e.Message);
            }
            Console.Out.WriteLine(JsonSerializer.Serialize(reply, JsonOptions));
            return reply.Ok ? 0 : 1;
        }

        private static Reply Fail(string code, string error) =>
            new() { Ok = false, State = "unknown", Code = code, Error = error };

        private static async Task<Reply> Run(Request req)
        {
            switch (req.Op)
            {
                case "start": return await Start(req);
                case "stop": return await Stop(req);
                case "status": return Status(req);
                default: return Fail("BadRequest", "unknown operation " + req.Op);
            }
        }

        // The manager is made from the connection profile of the uplink: that
        // is the connection Mobile Hotspot shares. When a specific adapter is
        // named it becomes the private (access point) side, which is the
        // overload that lets a USB dongle host while the built-in radio or
        // Ethernet is the uplink.
        private static NetworkOperatorTetheringManager? Manager(Request req, out Reply? failure)
        {
            failure = null;
            var profile = ProfileForAlias(req.Uplink);
            // Windows can remove the adapter/profile association after
            // Mobile Hotspot starts, even though the shared connection and
            // tethering manager remain active. Status and stop must then use
            // the current internet profile instead of reporting a false
            // failure. Start stays strict so a misspelled uplink is never
            // silently replaced with another connection.
            if (profile is null && req.Op != "start")
            {
                profile = NetworkInformation.GetInternetConnectionProfile();
            }
            if (profile is null)
            {
                failure = Fail("NoConnectionProfile",
                    "no connection profile uses the interface " + (req.Uplink ?? "(none)") +
                    "; Windows can only share a connection it has a profile for");
                return null;
            }
            var capability = NetworkOperatorTetheringManager.GetTetheringCapabilityFromConnectionProfile(profile);
            if (capability != TetheringCapability.Enabled)
            {
                failure = Fail(capability.ToString(), "Mobile Hotspot is not available: " + capability);
                return null;
            }
            if (!string.IsNullOrEmpty(req.Adapter))
            {
                var adapter = AdapterForAlias(req.Adapter);
                if (adapter is null)
                {
                    // An idle Wi-Fi adapter has no connection profile, so
                    // WinRT cannot give us its NetworkAdapter object. If the
                    // OS still knows the alias, use the default overload and
                    // let Windows select that available hotspot adapter.
                    if (GuidForAlias(req.Adapter) is not null)
                    {
                        return NetworkOperatorTetheringManager.CreateFromConnectionProfile(profile);
                    }
                    failure = Fail("NoAdapter", "no network adapter is called " + req.Adapter);
                    return null;
                }
                return NetworkOperatorTetheringManager.CreateFromConnectionProfile(profile, adapter);
            }
            return NetworkOperatorTetheringManager.CreateFromConnectionProfile(profile);
        }

        private static async Task<Reply> Start(Request req)
        {
            var manager = Manager(req, out var failure);
            if (manager is null) return failure!;

            var config = manager.GetCurrentAccessPointConfiguration();
            config.Ssid = req.Ssid ?? config.Ssid;
            config.Passphrase = req.Passphrase ?? config.Passphrase;
            var band = BandFor(req.Band);
            // IsBandSupported can report false when Windows selected an idle
            // adapter through the default manager overload. Configure the
            // requested band and let ConfigureAccessPointAsync return the
            // authoritative result. The panel still offers both bands and
            // the planner keeps 2.4 GHz as its compatibility-first default.
            config.Band = band;
            await manager.ConfigureAccessPointAsync(config);

            // Windows switches the hotspot off five minutes after the last
            // client leaves. A box that goes dark because nobody was joined
            // for a while is a box the user cannot rejoin, so the timeout is
            // disabled on every start rather than trusted to have stayed off.
            if (NetworkOperatorTetheringManager.IsNoConnectionsTimeoutEnabled())
            {
                await NetworkOperatorTetheringManager.DisableNoConnectionsTimeoutAsync();
            }

            if (manager.TetheringOperationalState == TetheringOperationalState.On)
            {
                return Describe(manager, true, null);
            }
            var result = await manager.StartTetheringAsync();
            if (result.Status != TetheringOperationStatus.Success)
            {
                return Fail(result.Status.ToString(),
                    "StartTetheringAsync: " + result.Status + (string.IsNullOrEmpty(result.AdditionalErrorMessage) ? "" : ": " + result.AdditionalErrorMessage));
            }
            return Describe(manager, true, null);
        }

        private static async Task<Reply> Stop(Request req)
        {
            var manager = Manager(req, out var failure);
            if (manager is null)
            {
                // With no shareable uplink there is nothing to stop; say off.
                return new Reply { Ok = true, State = "off" };
            }
            if (manager.TetheringOperationalState == TetheringOperationalState.Off)
            {
                return new Reply { Ok = true, State = "off" };
            }
            var result = await manager.StopTetheringAsync();
            if (result.Status != TetheringOperationStatus.Success)
            {
                return Fail(result.Status.ToString(), "StopTetheringAsync: " + result.Status);
            }
            return new Reply { Ok = true, State = "off" };
        }

        private static Reply Status(Request req)
        {
            var manager = Manager(req, out var failure);
            if (manager is null) return failure!;
            return Describe(manager, true, null);
        }

        private static Reply Describe(NetworkOperatorTetheringManager manager, bool ok, string? error)
        {
            var reply = new Reply { Ok = ok, Error = error };
            reply.State = manager.TetheringOperationalState switch
            {
                TetheringOperationalState.On => "on",
                TetheringOperationalState.Off => "off",
                TetheringOperationalState.InTransition => "transition",
                _ => "unknown",
            };
            var config = manager.GetCurrentAccessPointConfiguration();
            reply.Ssid = config.Ssid;
            reply.Band = config.Band switch
            {
                TetheringWiFiBand.TwoPointFourGigahertz => "2.4",
                TetheringWiFiBand.FiveGigahertz => "5",
                _ => "auto",
            };
            if (reply.State == "on")
            {
                reply.Clients = manager.GetTetheringClients()
                    .Select(c => new Client
                    {
                        Mac = c.MacAddress,
                        Hostnames = c.HostNames.Select(h => h.DisplayName).ToList(),
                    })
                    .ToList();
            }
            return reply;
        }

        private static TetheringWiFiBand BandFor(string? band) => band switch
        {
            "2.4" => TetheringWiFiBand.TwoPointFourGigahertz,
            "5" => TetheringWiFiBand.FiveGigahertz,
            _ => TetheringWiFiBand.Auto,
        };

        // Aliases are what the Go side names adapters by (Go's net package
        // uses the friendly name on Windows). WinRT names them by GUID, and
        // System.Net.NetworkInformation carries both, so it is the bridge.
        private static Guid? GuidForAlias(string? alias)
        {
            if (string.IsNullOrEmpty(alias)) return null;
            foreach (var nic in NetworkInterface.GetAllNetworkInterfaces())
            {
                if (string.Equals(nic.Name, alias, StringComparison.OrdinalIgnoreCase) && Guid.TryParse(nic.Id, out var id))
                {
                    return id;
                }
            }
            return null;
        }

        private static ConnectionProfile? ProfileForAlias(string? alias)
        {
            var id = GuidForAlias(alias);
            if (id is null) return null;
            foreach (var profile in NetworkInformation.GetConnectionProfiles())
            {
                if (profile.NetworkAdapter is not null && profile.NetworkAdapter.NetworkAdapterId == id.Value)
                {
                    return profile;
                }
            }
            return null;
        }

        private static NetworkAdapter? AdapterForAlias(string alias)
        {
            var id = GuidForAlias(alias);
            if (id is null) return null;
            // A Wi-Fi adapter that is not joined to anything has no connection
            // profile, so it is looked for among the LAN identifiers as well.
            foreach (var profile in NetworkInformation.GetConnectionProfiles())
            {
                if (profile.NetworkAdapter is not null && profile.NetworkAdapter.NetworkAdapterId == id.Value)
                {
                    return profile.NetworkAdapter;
                }
            }
            foreach (var lan in NetworkInformation.GetLanIdentifiers())
            {
                if (lan.NetworkAdapterId == id.Value)
                {
                    // LanIdentifier carries the id only; the adapter object
                    // itself comes from a profile. Without one Windows picks
                    // the adapter, which is the behaviour the panel explains.
                    return null;
                }
            }
            return null;
        }
    }
}
