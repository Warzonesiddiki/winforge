using Dapper;
using Newtonsoft.Json;
using System.Data;

namespace WinForge.Elite.Data
{
    /// <summary>
    /// Dapper type handler that maps JSON-encoded TEXT columns (e.g. Tweaks.Tags,
    /// Presets.IncludedTweakIds) to List&lt;string&gt; entity properties and back.
    /// Registered once in DbConnectionFactory's static constructor.
    /// </summary>
    public sealed class JsonStringListTypeHandler : SqlMapper.TypeHandler<List<string>>
    {
        public override void SetValue(IDbDataParameter parameter, List<string>? value)
        {
            parameter.Value = value is null ? null : JsonConvert.SerializeObject(value);
        }

        public override List<string> Parse(object value)
        {
            if (value is string json && !string.IsNullOrWhiteSpace(json))
            {
                try
                {
                    return JsonConvert.DeserializeObject<List<string>>(json) ?? new List<string>();
                }
                catch (JsonException)
                {
                    // Defensive: a malformed legacy value must never take down a query.
                    return new List<string>();
                }
            }

            return new List<string>();
        }
    }
}
