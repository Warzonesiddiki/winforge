using Microsoft.Win32;
using System.Globalization;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// A single catalog operation, deserialized from the JSON stored in the
    /// Tweaks/PrivacyRules tables. Supported operation types:
    ///   { "type": "registry",  "hive": "HKLM|HKCU", "key": "...", "valueName": "...", "kind": "DWord|QWord|String|ExpandString|MultiString|Binary", "data": "..." }
    ///   { "type": "command",   "command": "powercfg ..." }
    ///   { "type": "service",   "name": "SysMain", "startMode": "Disabled|Manual|Automatic|Automatic (Delayed)" }
    ///   { "type": "scheduledTask", "taskPath": @"Microsoft\Windows\...", "action": "Disable|Enable" }
    /// </summary>
    public sealed class Operation
    {
        public string Type { get; set; } = "registry";
        public string? Hive { get; set; }
        public string? Key { get; set; }
        public string? ValueName { get; set; }
        public string? Kind { get; set; }
        public string? Data { get; set; }
        public string? Command { get; set; }
        public string? Name { get; set; }
        public string? StartMode { get; set; }
        public string? TaskPath { get; set; }
        public string? Action { get; set; }
    }

    /// <summary>A captured registry value (name + kind + serialized data).</summary>
    public sealed class ValueSnapshot
    {
        public string Name { get; set; } = string.Empty;
        public string Kind { get; set; } = string.Empty;
        public string? Data { get; set; }
    }

    /// <summary>A captured registry key subtree used for undo/recovery.</summary>
    public sealed class KeySnapshot
    {
        public string Hive { get; set; } = string.Empty;
        public string KeyPath { get; set; } = string.Empty;
        public bool Exists { get; set; }
        public List<ValueSnapshot> Values { get; set; } = new();
        public List<KeySnapshot> SubKeys { get; set; } = new();
    }

    /// <summary>
    /// Read/write/delete access to the Windows registry with undo support.
    /// All mutations run on the thread pool, validate the hive, and refuse to
    /// touch protected system paths. Snapshots are serialized with JSON for
    /// on-disk recovery and restore.
    /// </summary>
    public sealed class RegistryService
    {
        private static readonly Serilog.ILogger Log = Logging.Logger.GetLogger<RegistryService>();

        /// <summary>Registry paths that must never be modified by the application.</summary>
        private static readonly string[] ProtectedPrefixes =
        {
            @"HKLM\SAM",
            @"HKLM\SECURITY",
            @"HKLM\SYSTEM\CurrentControlSet\Control\SafeBoot"
        };

        private const int MaxSnapshotDepth = 8;

        private static RegistryKey HiveKey(string hive)
        {
            return hive switch
            {
                "HKLM" => Registry.LocalMachine,
                "HKCU" => Registry.CurrentUser,
                _ => throw new NotSupportedException($"Registry hive '{hive}' is not supported. Supported hives: HKLM, HKCU.")
            };
        }

        private static void EnsureWritablePath(string hive, string keyPath)
        {
            var fullPath = $@"{hive}\{keyPath}";
            foreach (var prefix in ProtectedPrefixes)
            {
                if (fullPath.StartsWith(prefix, StringComparison.OrdinalIgnoreCase))
                {
                    throw new InvalidOperationException($"'{fullPath}' is a protected registry path and cannot be modified.");
                }
            }
        }

        public bool KeyExists(string hive, string keyPath)
        {
            try
            {
                using var key = HiveKey(hive).OpenSubKey(keyPath);
                return key != null;
            }
            catch (Exception ex)
            {
                Log.Debug(ex, "KeyExists failed for {Hive}\\{KeyPath}", hive, keyPath);
                return false;
            }
        }

        public bool ValueExists(string hive, string keyPath, string? valueName)
        {
            try
            {
                using var key = HiveKey(hive).OpenSubKey(keyPath);
                return key != null && key.GetValue(valueName) != null;
            }
            catch (Exception ex)
            {
                Log.Debug(ex, "ValueExists failed for {Hive}\\{KeyPath}\\{Value}", hive, keyPath, valueName ?? "(default)");
                return false;
            }
        }

        /// <summary>Reads a raw registry value (environment variables are NOT expanded).</summary>
        public object? ReadValue(string hive, string keyPath, string? valueName)
        {
            using var key = HiveKey(hive).OpenSubKey(keyPath);
            return key?.GetValue(valueName, null, RegistryValueOptions.DoNotExpandEnvironmentNames);
        }

        /// <summary>Reads a DWord/QWord value as a long, or null when the value does not exist.</summary>
        public long? ReadDWord(string hive, string keyPath, string? valueName)
        {
            var value = ReadValue(hive, keyPath, valueName);
            if (value is null)
            {
                return null;
            }

            try
            {
                return Convert.ToInt64(value, CultureInfo.InvariantCulture);
            }
            catch (Exception ex)
            {
                Log.Debug(ex, "ReadDWord failed to convert {Hive}\\{KeyPath}\\{Value}", hive, keyPath, valueName ?? "(default)");
                return null;
            }
        }

        public Task WriteValueAsync(string hive, string keyPath, string? valueName, string kind, string? data, CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                ct.ThrowIfCancellationRequested();
                EnsureWritablePath(hive, keyPath);
                using var key = HiveKey(hive).CreateSubKey(keyPath, writable: true)
                    ?? throw new InvalidOperationException($"Failed to open or create registry key {hive}\\{keyPath}.");
                key.SetValue(valueName, ParseData(kind, data), ParseKind(kind));
                Log.Information("Registry write: {Hive}\\{Key}\\{Value} = {Data} ({Kind})", hive, keyPath, valueName ?? "(default)", data ?? string.Empty, kind);
            }, ct);
        }

        public Task DeleteValueAsync(string hive, string keyPath, string? valueName, CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                ct.ThrowIfCancellationRequested();
                EnsureWritablePath(hive, keyPath);
                using var key = HiveKey(hive).OpenSubKey(keyPath, writable: true);
                if (key is null)
                {
                    return;
                }

                key.DeleteValue(valueName ?? string.Empty, throwOnMissingValue: false);
                Log.Information("Registry delete value: {Hive}\\{Key}\\{Value}", hive, keyPath, valueName ?? "(default)");
            }, ct);
        }

        public Task DeleteKeyTreeAsync(string hive, string keyPath, CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                ct.ThrowIfCancellationRequested();
                EnsureWritablePath(hive, keyPath);
                HiveKey(hive).DeleteSubKeyTree(keyPath, throwOnMissingSubKey: false);
                Log.Information("Registry delete key: {Hive}\\{Key}", hive, keyPath);
            }, ct);
        }

        /// <summary>Captures the current state of every registry key touched by the given operations.</summary>
        public Task<List<KeySnapshot>> CaptureSnapshotAsync(IReadOnlyList<Operation> operations, CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                ct.ThrowIfCancellationRequested();
                var captured = new Dictionary<string, KeySnapshot>(StringComparer.OrdinalIgnoreCase);
                foreach (var operation in operations)
                {
                    if (operation.Type != "registry" || string.IsNullOrWhiteSpace(operation.Hive) || string.IsNullOrWhiteSpace(operation.Key))
                    {
                        continue;
                    }

                    var mapKey = $"{operation.Hive}\\{operation.Key}";
                    if (!captured.ContainsKey(mapKey))
                    {
                        captured[mapKey] = CaptureKey(operation.Hive, operation.Key, 0);
                    }
                }

                return captured.Values.ToList();
            }, ct);
        }

        /// <summary>
        /// Restores previously captured values. Conservative by design: captured values are
        /// written back, but values and keys that did not exist at snapshot time are not
        /// deleted, so a restore can never destroy data added after the snapshot.
        /// </summary>
        public Task RestoreSnapshotAsync(IReadOnlyList<KeySnapshot> snapshots, CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                ct.ThrowIfCancellationRequested();
                foreach (var snapshot in snapshots)
                {
                    RestoreKey(snapshot);
                }
            }, ct);
        }

        /// <summary>Verifies that a registry operation produced the expected value.</summary>
        public Task<bool> VerifyAsync(Operation operation, CancellationToken ct = default)
        {
            return Task.Run(() =>
            {
                ct.ThrowIfCancellationRequested();
                if (string.IsNullOrWhiteSpace(operation.Hive) || string.IsNullOrWhiteSpace(operation.Key))
                {
                    return false;
                }

                var actual = ReadValue(operation.Hive, operation.Key, operation.ValueName);
                var expected = ParseData(operation.Kind ?? "String", operation.Data);
                return ValuesEqual(actual, expected);
            }, ct);
        }

        private static KeySnapshot CaptureKey(string hive, string keyPath, int depth)
        {
            var snapshot = new KeySnapshot { Hive = hive, KeyPath = keyPath };
            using var key = HiveKey(hive).OpenSubKey(keyPath);
            if (key is null)
            {
                snapshot.Exists = false;
                return snapshot;
            }

            snapshot.Exists = true;
            foreach (var valueName in key.GetValueNames())
            {
                try
                {
                    var kind = key.GetValueKind(valueName);
                    if (kind == RegistryValueKind.Unknown)
                    {
                        continue;
                    }

                    var raw = key.GetValue(valueName, null, RegistryValueOptions.DoNotExpandEnvironmentNames);
                    var data = raw switch
                    {
                        null => null,
                        byte[] bytes => Convert.ToBase64String(bytes),
                        string[] multi => string.Join("\u0000", multi),
                        _ => raw.ToString()
                    };
                    snapshot.Values.Add(new ValueSnapshot { Name = valueName, Kind = kind.ToString(), Data = data });
                }
                catch (Exception ex)
                {
                    Log.Warning(ex, "Failed to capture registry value {Name} at {Hive}\\{Key}", valueName, hive, keyPath);
                }
            }

            if (depth < MaxSnapshotDepth)
            {
                foreach (var subKeyName in key.GetSubKeyNames())
                {
                    snapshot.SubKeys.Add(CaptureKey(hive, keyPath + "\\" + subKeyName, depth + 1));
                }
            }

            return snapshot;
        }

        private static void RestoreKey(KeySnapshot snapshot)
        {
            if (!snapshot.Exists)
            {
                return; // Key did not exist at snapshot time; never delete (conservative restore).
            }

            EnsureWritablePath(snapshot.Hive, snapshot.KeyPath);
            using var key = HiveKey(snapshot.Hive).CreateSubKey(snapshot.KeyPath, writable: true);
            if (key is null)
            {
                Log.Warning("Failed to open key for restore: {Hive}\\{Key}", snapshot.Hive, snapshot.KeyPath);
                return;
            }

            foreach (var value in snapshot.Values)
            {
                try
                {
                    key.SetValue(value.Name, ParseData(value.Kind, value.Data), ParseKind(value.Kind));
                }
                catch (Exception ex)
                {
                    Log.Warning(ex, "Failed to restore value {Name} at {Hive}\\{Key}", value.Name, snapshot.Hive, snapshot.KeyPath);
                }
            }

            foreach (var subKey in snapshot.SubKeys)
            {
                RestoreKey(subKey);
            }
        }

        public static RegistryValueKind ParseKind(string kind)
        {
            return kind switch
            {
                "DWord" => RegistryValueKind.DWord,
                "QWord" => RegistryValueKind.QWord,
                "String" => RegistryValueKind.String,
                "ExpandString" => RegistryValueKind.ExpandString,
                "MultiString" => RegistryValueKind.MultiString,
                "Binary" => RegistryValueKind.Binary,
                _ => throw new InvalidOperationException($"Unknown registry value kind '{kind}'.")
            };
        }

        public static object? ParseData(string kind, string? data)
        {
            return kind switch
            {
                "DWord" => ParseUInt32(data),
                "QWord" => ParseUInt64(data),
                "String" or "ExpandString" => data ?? string.Empty,
                "MultiString" => string.IsNullOrEmpty(data)
                    ? Array.Empty<string>()
                    : data.Split('\u0000', StringSplitOptions.RemoveEmptyEntries),
                "Binary" => string.IsNullOrEmpty(data) ? Array.Empty<byte>() : Convert.FromHexString(data),
                _ => throw new InvalidOperationException($"Unknown registry value kind '{kind}'.")
            };
        }

        private static uint ParseUInt32(string? data)
        {
            if (string.IsNullOrWhiteSpace(data))
            {
                throw new InvalidOperationException("DWord data is missing.");
            }

            var text = data.Trim();
            if (text.StartsWith("0x", StringComparison.OrdinalIgnoreCase))
            {
                return Convert.ToUInt32(text[2..], 16);
            }

            return uint.Parse(text, NumberStyles.Integer, CultureInfo.InvariantCulture);
        }

        private static ulong ParseUInt64(string? data)
        {
            if (string.IsNullOrWhiteSpace(data))
            {
                throw new InvalidOperationException("QWord data is missing.");
            }

            var text = data.Trim();
            if (text.StartsWith("0x", StringComparison.OrdinalIgnoreCase))
            {
                return Convert.ToUInt64(text[2..], 16);
            }

            return ulong.Parse(text, NumberStyles.Integer, CultureInfo.InvariantCulture);
        }

        private static bool ValuesEqual(object? actual, object? expected)
        {
            if (actual is null && expected is null)
            {
                return true;
            }

            if (actual is null || expected is null)
            {
                return false;
            }

            if (actual is byte[] actualBytes && expected is byte[] expectedBytes)
            {
                return actualBytes.SequenceEqual(expectedBytes);
            }

            if (actual is string[] actualMulti && expected is string[] expectedMulti)
            {
                return actualMulti.SequenceEqual(expectedMulti);
            }

            if (actual is int actualInt && expected is uint expectedUInt)
            {
                return actualInt == (long)expectedUInt;
            }

            if (actual is long actualLong && expected is ulong expectedULong)
            {
                return actualLong == (long)expectedULong;
            }

            return string.Equals(
                Convert.ToString(actual, CultureInfo.InvariantCulture),
                Convert.ToString(expected, CultureInfo.InvariantCulture),
                StringComparison.OrdinalIgnoreCase);
        }
    }
}
