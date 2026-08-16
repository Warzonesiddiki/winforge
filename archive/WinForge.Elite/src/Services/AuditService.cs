using Dapper;
using System.Data;
using WinForge.Elite.Models;

namespace WinForge.Elite.Services
{
    /// <summary>
    /// Writes operation history entries to the local audit database. Every mutation
    /// performed by WinForge Elite flows through this logger so the History view and
    /// undo flows have a complete trail.
    /// </summary>
    public static class AuditService
    {
        public static int Log(
            IDbConnection connection,
            IDbTransaction? transaction,
            OperationType type,
            string operationName,
            string details,
            string? undoPayload = null,
            bool success = true,
            string? errorMessage = null,
            int? restorePointId = null)
        {
            var executedAt = DateTime.UtcNow.ToString("o");
            connection.Execute(
                @"INSERT INTO OperationHistory
                      (OperationType, OperationName, Details, UndoPayload, Success, ErrorMessage, ExecutedAt, RestorePointId)
                  VALUES
                      (@OperationType, @OperationName, @Details, @UndoPayload, @Success, @ErrorMessage, @ExecutedAt, @RestorePointId)",
                new
                {
                    OperationType = (int)type,
                    OperationName = operationName,
                    Details = details,
                    UndoPayload = undoPayload,
                    Success = success ? 1 : 0,
                    ErrorMessage = errorMessage,
                    ExecutedAt = executedAt,
                    RestorePointId = restorePointId
                },
                transaction);

            return connection.ExecuteScalar<int>("SELECT last_insert_rowid();", transaction: transaction);
        }
    }
}
