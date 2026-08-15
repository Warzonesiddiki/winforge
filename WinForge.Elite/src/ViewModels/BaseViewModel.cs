using System.ComponentModel;
using System.Runtime.CompilerServices;

namespace WinForge.Elite.ViewModels
{
    /// <summary>
    /// Base class for all view models. Provides INotifyPropertyChanged plumbing,
    /// busy-state tracking, and a guarded async execution helper that surfaces
    /// errors to the UI and the application log.
    /// </summary>
    public abstract class BaseViewModel : INotifyPropertyChanged
    {
        private bool _isBusy;
        private string _statusMessage = "Ready";
        private string? _errorMessage;

        public event PropertyChangedEventHandler? PropertyChanged;

        /// <summary>True while a background operation is in flight. UI binds controls to this.</summary>
        public bool IsBusy
        {
            get => _isBusy;
            private set => SetProperty(ref _isBusy, value);
        }

        /// <summary>Human-readable status line shown in the UI (last success or failure).</summary>
        public string StatusMessage
        {
            get => _statusMessage;
            private set => SetProperty(ref _statusMessage, value);
        }

        /// <summary>Set when the last operation failed; null otherwise.</summary>
        public string? ErrorMessage
        {
            get => _errorMessage;
            private set => SetProperty(ref _errorMessage, value);
        }

        protected bool SetProperty<T>(ref T field, T value, [CallerMemberName] string? propertyName = null)
        {
            if (EqualityComparer<T>.Default.Equals(field, value))
            {
                return false;
            }

            field = value;
            OnPropertyChanged(propertyName);
            return true;
        }

        protected virtual void OnPropertyChanged([CallerMemberName] string? propertyName = null)
        {
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
        }

        protected void SetStatus(string message)
        {
            StatusMessage = message;
            ErrorMessage = null;
        }

        protected void SetError(string message)
        {
            ErrorMessage = message;
            StatusMessage = message;
        }

        /// <summary>
        /// Runs <paramref name="work"/> with busy-state tracking. Exceptions are caught,
        /// logged, and exposed through <see cref="ErrorMessage"/>/<see cref="StatusMessage"/>
        /// instead of crashing the UI thread. Re-entrant calls are ignored while busy.
        /// </summary>
        protected async Task RunBusyAsync(Func<Task> work, string errorContext)
        {
            if (IsBusy)
            {
                return;
            }

            IsBusy = true;
            try
            {
                await work().ConfigureAwait(true);
            }
            catch (Exception ex)
            {
                Logging.Logger.GetLogger(GetType().Name).Error(ex, errorContext);
                SetError($"{errorContext}: {ex.Message}");
            }
            finally
            {
                IsBusy = false;
            }
        }
    }
}
