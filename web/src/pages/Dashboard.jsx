import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiService } from '../services/api';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { useToast } from '../components/Toast';
import { getAuthToken } from '../utils/storage';

export const Dashboard = () => {
  const navigate = useNavigate();
  const [scanStatus, setScanStatus] = useState(null);
  const [scanProgress, setScanProgress] = useState(null);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [folders, setFolders] = useState([]);
  const [selectedFolders, setSelectedFolders] = useState(['INBOX']);
  const [showFolderSelector, setShowFolderSelector] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const { toasts, addToast } = useToast();

  // Verificar autenticação
  useEffect(() => {
    const token = getAuthToken();
    if (!token) {
      navigate('/login');
      return;
    }
    setIsAuthenticated(true);
  }, [navigate]);

  const fetchData = async () => {
    if (!isAuthenticated) return;
    
    try {
      setLoading(true);
      const [statusData, statsData] = await Promise.all([
        apiService.getScanStatus(),
        apiService.getStats(),
      ]);
      setScanStatus(statusData);
      setStats(statsData);

      // Se estiver escaneando, buscar progresso
      if (statusData?.is_running) {
        const progressData = await apiService.getScanProgress();
        setScanProgress(progressData);
        setScanning(true);
      } else {
        setScanning(false);
      }
    } catch (error) {
      console.error('Error fetching data:', error);
      // Se erro 401, redirecionar para login
      if (error.response?.status === 401) {
        navigate('/login');
      }
    } finally {
      setLoading(false);
    }
  };

  const fetchFolders = async () => {
    if (!isAuthenticated) return;
    
    try {
      const data = await apiService.getFolders();
      setFolders(data.folders || []);
    } catch (error) {
      console.error('Error fetching folders:', error);
      if (error.response?.status !== 401) {
        addToast('Erro ao carregar pastas', 'error');
      }
    }
  };

  useEffect(() => {
    if (!isAuthenticated) return;
    
    fetchData();
    fetchFolders();
    
    // Atualizar a cada 2 segundos durante varredura, 10 segundos quando idle
    const interval = setInterval(() => {
      fetchData();
    }, scanning ? 2000 : 10000);
    
    return () => clearInterval(interval);
  }, [scanning, isAuthenticated]);

  const handleManualScan = async () => {
    if (selectedFolders.length === 0) {
      addToast('Selecione pelo menos uma pasta para escanear', 'error');
      return;
    }

    try {
      setScanning(true);
      await apiService.startScan(selectedFolders);
      addToast(`Varredura iniciada em ${selectedFolders.length} pasta(s)!`, 'success');
      setShowFolderSelector(false);
      // Atualizar status imediatamente
      setTimeout(() => fetchData(), 1000);
    } catch (error) {
      addToast('Erro ao iniciar varredura: ' + error.message, 'error');
      setScanning(false);
    }
  };

  const handleCancelScan = async () => {
    try {
      await apiService.cancelScan();
      addToast('Cancelamento solicitado...', 'info');
      setTimeout(() => fetchData(), 1000);
    } catch (error) {
      addToast('Erro ao cancelar varredura: ' + error.message, 'error');
    }
  };

  const toggleFolder = (folder) => {
    setSelectedFolders((prev) =>
      prev.includes(folder)
        ? prev.filter((f) => f !== folder)
        : [...prev, folder]
    );
  };

  const formatDate = (dateString) => {
    if (!dateString) return 'Nunca';
    try {
      return new Date(dateString).toLocaleString('pt-BR');
    } catch {
      return 'Inválido';
    }
  };

  if (loading) {
    return <LoadingSpinner message="Carregando dashboard..." />;
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      {/* Toasts */}
      <div className="fixed top-4 right-4 space-y-2 z-40">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className={`${
              toast.type === 'success'
                ? 'bg-green-500'
                : toast.type === 'error'
                ? 'bg-red-500'
                : 'bg-blue-500'
            } text-white px-4 py-3 rounded-lg shadow-lg`}
          >
            {toast.message}
          </div>
        ))}
      </div>

      <div className="mb-8">
        <h1 className="text-4xl font-bold text-gray-900 mb-2">Dashboard</h1>
        <p className="text-gray-600">Gerencie suas varreduras de e-mail do Gmail</p>
      </div>

      {/* Status Card */}
      <div className="bg-white rounded-lg shadow-lg p-8 mb-8">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold text-gray-900">Status da Varredura</h2>
          {scanStatus?.is_running && (
            <span className="flex items-center gap-2 text-orange-600">
              <div className="animate-pulse h-3 w-3 bg-orange-600 rounded-full"></div>
              Escaneando...
            </span>
          )}
        </div>

        {/* Barra de Progresso */}
        {scanStatus?.is_running && scanProgress && (
          <div className="mb-6 bg-blue-50 p-6 rounded-lg border border-blue-200">
            <div className="flex items-center justify-between mb-3">
              <div>
                <p className="text-sm font-semibold text-gray-700">
                  {scanProgress.status === 'connecting' && 'Conectando...'}
                  {scanProgress.status === 'scanning' && `Escaneando: ${scanProgress.current_folder}`}
                  {scanProgress.status === 'completed' && 'Concluído!'}
                  {scanProgress.status === 'cancelled' && 'Cancelado'}
                  {scanProgress.status === 'error' && 'Erro'}
                </p>
                <p className="text-xs text-gray-600 mt-1">
                  Pasta {scanProgress.folders_processed} de {scanProgress.folders_total} • 
                  {scanProgress.emails_processed} emails processados
                </p>
              </div>
              <span className="text-2xl font-bold text-primary-600">
                {scanProgress.percent_complete}%
              </span>
            </div>

            {/* Barra de progresso visual */}
            <div className="w-full bg-gray-200 rounded-full h-4 overflow-hidden">
              <div
                className="bg-gradient-to-r from-primary-500 to-primary-600 h-4 rounded-full transition-all duration-500 ease-out"
                style={{ width: `${scanProgress.percent_complete}%` }}
              ></div>
            </div>

            {/* Botão Cancelar */}
            {scanProgress.status === 'scanning' && (
              <button
                onClick={handleCancelScan}
                className="mt-4 w-full bg-red-600 text-white px-4 py-2 rounded-lg font-semibold hover:bg-red-700 transition"
              >
                ⏹ Interromper Varredura
              </button>
            )}
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
          <div className="bg-gray-50 p-4 rounded-lg">
            <p className="text-sm text-gray-600 mb-2">Última Varredura</p>
            <p className="text-lg font-semibold text-gray-900">{formatDate(scanStatus?.last_scan_time)}</p>
            {scanStatus?.last_emails_scanned > 0 && (
              <p className="text-sm text-gray-500 mt-1">{scanStatus.last_emails_scanned} e-mails processados</p>
            )}
          </div>

          <div className="bg-gray-50 p-4 rounded-lg">
            <p className="text-sm text-gray-600 mb-2">Status do Sistema</p>
            <p className="text-lg font-semibold text-gray-900">
              {scanStatus?.is_running ? '⚡ Escaneando...' : '✅ Pronto'}
            </p>
            <p className="text-sm text-gray-500 mt-1">Varredura manual sob demanda</p>
          </div>
        </div>

        {scanStatus?.last_error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-6">
            <p className="text-sm text-red-800">
              <strong>Último Erro:</strong> {scanStatus.last_error}
            </p>
          </div>
        )}

        {/* Seletor de Pastas */}
        {!scanStatus?.is_running && (
          <>
            <button
              onClick={() => setShowFolderSelector(!showFolderSelector)}
              className="w-full md:w-auto bg-gray-600 text-white px-6 py-3 rounded-lg font-semibold hover:bg-gray-700 transition mb-4"
            >
              {showFolderSelector ? '▼ Ocultar Seleção de Pastas' : '▶ Selecionar Pastas para Escanear'}
            </button>

            {showFolderSelector && (
              <div className="bg-gray-50 p-6 rounded-lg mb-4 border border-gray-200">
                <p className="text-sm font-semibold text-gray-700 mb-3">
                  Selecione as pastas que deseja escanear:
                </p>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3 max-h-64 overflow-y-auto">
                  {folders.map((folder) => (
                    <label
                      key={folder}
                      className="flex items-center gap-2 bg-white p-3 rounded-lg border border-gray-300 hover:border-primary-500 cursor-pointer transition"
                    >
                      <input
                        type="checkbox"
                        checked={selectedFolders.includes(folder)}
                        onChange={() => toggleFolder(folder)}
                        className="h-4 w-4 text-primary-600 rounded focus:ring-primary-500"
                      />
                      <span className="text-sm text-gray-700">{folder}</span>
                    </label>
                  ))}
                </div>
                <p className="text-xs text-gray-500 mt-3">
                  {selectedFolders.length} pasta(s) selecionada(s)
                </p>
              </div>
            )}

            <button
              onClick={handleManualScan}
              disabled={scanning || selectedFolders.length === 0}
              className="w-full md:w-auto bg-primary-600 text-white px-6 py-3 rounded-lg font-semibold hover:bg-primary-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition"
            >
              🚀 Iniciar Varredura Manual
            </button>
          </>
        )}
      </div>

      {/* Statistics */}
      {stats && (
        <div className="bg-white rounded-lg shadow-lg p-8">
          <h2 className="text-2xl font-bold text-gray-900 mb-6">Estatísticas</h2>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
            <div className="bg-blue-50 p-6 rounded-lg">
              <p className="text-sm text-gray-600 mb-2">Total de Artigos Extraídos</p>
              <p className="text-4xl font-bold text-primary-600">{stats.total_articles || 0}</p>
              <p className="text-xs text-gray-500 mt-2">Links encontrados nas newsletters</p>
            </div>

            <div className="bg-green-50 p-6 rounded-lg">
              <p className="text-sm text-gray-600 mb-2">Artigos Salvos Localmente</p>
              <p className="text-4xl font-bold text-green-600">{stats.total_imported || 0}</p>
              <p className="text-xs text-gray-500 mt-2">Importados para leitura offline</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
