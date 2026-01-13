#include <apt-pkg/init.h>
#include <apt-pkg/cachefile.h>
#include <apt-pkg/pkgcache.h>
#include <apt-pkg/algorithms.h>
#include <apt-pkg/pkgsystem.h>
#include <apt-pkg/progress.h>
#include <apt-pkg/error.h>
#include <apt-pkg/acquire.h>
#include <apt-pkg/upgrade.h>
#include <apt-pkg/depcache.h>
#include <apt-pkg/sourcelist.h>
#include <apt-pkg/policy.h>
#include <apt-pkg/pkgrecords.h>
#include <apt-pkg/hashes.h>

#include <iostream>
#include <vector>
#include <string>
#include <cstring>
#include <cstdlib>

#include "upgrade_query.h"

bool UpgradePackage::Valid() const {
    if (Name.empty() ||
        CandidateVersion.empty() ||
        Architecture.empty() ||
        Codename.empty() ||
        Site.empty() ||
        Filename.empty() ||
        Hash.empty()) {
        return false;
    }
    
    // Check uint64_t fields - they should not be 0
    if (Size == 0) {
        return false;
    }
    
    return true;
}

UpgradePackage GetUpgradePackage(pkgCacheFile &Cache, pkgRecords &Recs, const pkgCache::PkgIterator &pkg) {
    UpgradePackage result;
    pkgCache::VerIterator candVer = Cache->GetCandidateVersion(pkg);

    result.Name = pkg.Name();

    // Fill installed version if present
    pkgCache::VerIterator curVer = pkg.CurrentVer();
    if (!curVer.end()) {
        result.InstalledVersion = curVer.VerStr();
    }

    if (!candVer.end()) {
        result.CandidateVersion = candVer.VerStr();
        result.Architecture = candVer.Arch();
        result.Size = candVer->Size;
        result.InstalledSize = candVer->InstalledSize;

        // Get file list to find repository information
        pkgCache::VerFileIterator vf = candVer.FileList();
        if (!vf.end()) {
            result.Codename = vf.File().Codename() ? vf.File().Codename() : "";
            result.Component = vf.File().Component() ? vf.File().Component() : "";

            // Get download information
            pkgCache::PkgFileIterator pf = vf.File();
            if (!pf.end()) {
                result.Site = pf.Site() ? pf.Site() : "";
            }

            // Use pkgRecords to get detailed download information
            pkgRecords::Parser& parser = Recs.Lookup(vf);
            result.Filename = parser.FileName();

            // Get hash information
            HashStringList hashes = parser.Hashes();
            if (!hashes.empty()) {
                const HashString *hash = hashes.find(NULL);
                if (hash) {
                    result.Hash = hash->toStr();
                }
            }
        }
    }

    return result;
}

std::vector<UpgradePackage> GetUpgradePackages(const std::string &sourcelist, const std::string &sourceparts, bool allow_downgrades) {
    std::vector<UpgradePackage> result;
    if (!pkgInitConfig(*_config)) {
        std::cerr << "Failed to initialize APT config" << std::endl;
        return result;
    }

    if (!pkgInitSystem(*_config, _system)) {
        std::cerr << "Failed to initialize APT system" << std::endl;
        return result;
    }

    // disable debug output
    _config->CndSet("quiet::NoProgress", true);
    _config->Set("quiet", 1);

    _config->Set("APT::Get::allow-downgrades", allow_downgrades);

    // If custom paths are provided, set configuration items
    if (!sourcelist.empty()) {
        _config->Set("Dir::Etc::sourcelist", sourcelist);
    }
    if (!sourceparts.empty()) {
        _config->Set("Dir::Etc::sourceparts", sourceparts);
    }

    pkgCacheFile Cache;
    OpTextProgress Prog(*_config);
    if (!Cache.Open(&Prog, false)) {
        std::cerr << "Could not open cache" << std::endl;
        return result;
    }

    APT::Upgrade::Upgrade(*Cache, APT::Upgrade::ALLOW_EVERYTHING);
    Cache->MarkAndSweep();

    pkgRecords Recs(Cache);
    result.reserve(Cache->Head().PackageCount / 4); // Reserve approximate space

    for (pkgCache::PkgIterator pkg = Cache->PkgBegin(); !pkg.end(); ++pkg) {
        const pkgDepCache::StateCache& state = (*Cache)[pkg];

        if (state.NewInstall() || state.Upgrade() || state.Downgrade()) {
            result.emplace_back(GetUpgradePackage(Cache, Recs, pkg));
        }
    }
    
    return result;
}
