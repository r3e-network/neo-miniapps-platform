import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { WalletProvider } from './hooks/useWallet';
import { Header } from './components/Header';
import { HomePage } from './components/HomePage';
import { DepositPage } from './components/DepositPage';
import { WithdrawPage } from './components/WithdrawPage';
import { StatsPage } from './components/StatsPage';

function App() {
  return (
    <WalletProvider>
      <BrowserRouter>
        <div className="min-h-screen bg-gradient-to-br from-gray-900 via-purple-900 to-gray-900">
          <Header />
          <main className="container mx-auto px-4 py-8">
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/deposit" element={<DepositPage />} />
              <Route path="/withdraw" element={<WithdrawPage />} />
              <Route path="/stats" element={<StatsPage />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </WalletProvider>
  );
}

export default App;
